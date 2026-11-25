// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagebin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
)

// goBinary returns this test binary, a Go binary whose build information
// debug.ReadBuildInfo returns.
var goBinary = sync.OnceValues(func() ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(exe)
})

// store is a memory store that also finds what it holds by digest, as a
// registry does.
type store struct {
	*memory.Store

	mu      sync.Mutex
	digests map[digest.Digest]ocispec.Descriptor
}

func newStore() *store {
	return &store{Store: memory.New(), digests: map[digest.Digest]ocispec.Descriptor{}}
}

func (s *store) Push(ctx context.Context, expected ocispec.Descriptor, r io.Reader) error {
	if err := s.Store.Push(ctx, expected, r); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.digests[expected.Digest] = expected
	return nil
}

// find returns the descriptor of what s holds under ref, a tag or a digest.
func (s *store) find(ctx context.Context, ref string) (ocispec.Descriptor, bool) {
	s.mu.Lock()
	desc, ok := s.digests[digest.Digest(ref)]
	s.mu.Unlock()
	if ok {
		return desc, true
	}
	desc, err := s.Resolve(ctx, ref)
	return desc, err == nil
}

// tarEntry is an entry of a layer.
type tarEntry struct {
	name     string
	typeflag byte
	data     []byte
	linkname string
}

func regular(name string, data []byte) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeReg, data: data}
}

func symlink(name, target string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeSymlink, linkname: target}
}

func dir(name string) tarEntry {
	return tarEntry{name: name, typeflag: tar.TypeDir}
}

// layer is a layer of an image, with the media type of its compression.
type layer struct {
	mediaType string
	entries   []tarEntry
}

func uncompressed(entries ...tarEntry) layer {
	return layer{mediaType: ocispec.MediaTypeImageLayer, entries: entries}
}

func gzipped(entries ...tarEntry) layer {
	return layer{mediaType: ocispec.MediaTypeImageLayerGzip, entries: entries}
}

// imageSpec describes an image to push.
type imageSpec struct {
	platform     ocispec.Platform
	config       ocispec.ImageConfig
	layers       []layer
	dockerFormat bool
}

func push(t *testing.T, store *store, mediaType string, data []byte) ocispec.Descriptor {
	t.Helper()
	desc := content.NewDescriptorFromBytes(mediaType, data)
	if err := store.Push(t.Context(), desc, bytes.NewReader(data)); !errors.Is(err, errdef.ErrAlreadyExists) {
		require.NoError(t, err)
	}
	return desc
}

func pushJSON(t *testing.T, store *store, mediaType string, v any) ocispec.Descriptor {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return push(t, store, mediaType, data)
}

func layerBlob(t *testing.T, l layer) []byte {
	t.Helper()
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	for _, e := range l.entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typeflag, Linkname: e.linkname, Mode: 0o755, Size: int64(len(e.data))}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write(e.data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	var blob bytes.Buffer
	switch l.mediaType {
	case ocispec.MediaTypeImageLayerGzip, dockerLayerGzip:
		gw := gzip.NewWriter(&blob)
		_, err := gw.Write(archive.Bytes())
		require.NoError(t, err)
		require.NoError(t, gw.Close())
	case ocispec.MediaTypeImageLayerZstd:
		zw, err := zstd.NewWriter(&blob)
		require.NoError(t, err)
		_, err = zw.Write(archive.Bytes())
		require.NoError(t, err)
		require.NoError(t, zw.Close())
	default:
		return archive.Bytes()
	}
	return blob.Bytes()
}

// pushImage pushes the image spec describes and returns the descriptor of
// its manifest.
func pushImage(t *testing.T, store *store, spec imageSpec) ocispec.Descriptor {
	t.Helper()
	manifestType, configType := ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageConfig
	if spec.dockerFormat {
		manifestType, configType = dockerManifest, "application/vnd.docker.container.image.v1+json"
	}
	config := ocispec.Image{Platform: spec.platform, Config: spec.config}
	manifest := ocispec.Manifest{
		MediaType: manifestType,
		Config:    pushJSON(t, store, configType, config),
	}
	manifest.SchemaVersion = 2
	for _, l := range spec.layers {
		manifest.Layers = append(manifest.Layers, push(t, store, l.mediaType, layerBlob(t, l)))
	}
	desc := pushJSON(t, store, manifestType, manifest)
	desc.Platform = &spec.platform
	return desc
}

func pushIndex(t *testing.T, store *store, mediaType string, manifests ...ocispec.Descriptor) ocispec.Descriptor {
	t.Helper()
	index := ocispec.Index{MediaType: mediaType, Manifests: manifests}
	index.SchemaVersion = 2
	return pushJSON(t, store, mediaType, index)
}

var (
	amd64 = ocispec.Platform{OS: "linux", Architecture: "amd64"}
	arm64 = ocispec.Platform{OS: "linux", Architecture: "arm64"}
)

// provider is the configuration of the scaffolded provider image.
var provider = ocispec.ImageConfig{Entrypoint: []string{"/provider"}, WorkingDir: "/"}

func TestRead(t *testing.T) {
	t.Parallel()

	bin, err := goBinary()
	require.NoError(t, err)
	want, ok := debug.ReadBuildInfo()
	require.True(t, ok)

	// The base layer of a distroless image, then the binary.
	base := gzipped(dir("etc/"), regular("etc/passwd", []byte("nonroot:x:65532:65532::/home/nonroot:/sbin/nologin\n")))
	attestation := ocispec.Descriptor{
		MediaType:   ocispec.MediaTypeImageManifest,
		Digest:      digest.FromString("attestation"),
		Size:        1,
		Platform:    &ocispec.Platform{OS: "unknown", Architecture: "unknown"},
		Annotations: map[string]string{"vnd.docker.reference.type": "attestation-manifest"},
	}

	tests := map[string]struct {
		push          func(t *testing.T, store *store) ocispec.Descriptor
		wantPlatforms []string
		wantPath      string
	}{
		"multi-platform OCI index with an attestation": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushIndex(t, store, ocispec.MediaTypeImageIndex,
					pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{base, uncompressed(regular("provider", bin))}}),
					pushImage(t, store, imageSpec{platform: arm64, config: provider, layers: []layer{base, uncompressed(regular("provider", bin))}}),
					attestation,
				)
			},
			wantPlatforms: []string{"linux/amd64", "linux/arm64"},
			wantPath:      "/provider",
		},
		"Docker manifest list": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushIndex(t, store, dockerManifestList,
					pushImage(t, store, imageSpec{platform: amd64, config: provider, dockerFormat: true, layers: []layer{
						{mediaType: dockerLayerGzip, entries: []tarEntry{regular("./provider", bin)}},
					}}),
				)
			},
			wantPlatforms: []string{"linux/amd64"},
			wantPath:      "/provider",
		},
		"single image, binary on the PATH behind a link": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{
					platform: arm64,
					config:   ocispec.ImageConfig{Cmd: []string{"manager", "--leader-elect"}, Env: []string{"PATH=/usr/local/bin:/usr/bin"}},
					layers: []layer{
						{mediaType: ocispec.MediaTypeImageLayerZstd, entries: []tarEntry{regular("opt/manager", bin)}},
						uncompressed(symlink("usr/bin/manager", "../../opt/manager")),
					},
				})
			},
			wantPlatforms: []string{"linux/arm64"},
			wantPath:      "/opt/manager",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newStore()
			desc := tt.push(t, store)
			require.NoError(t, store.Tag(t.Context(), desc, "1.0.0"))

			got, err := Read(t.Context(), store, "1.0.0")

			require.NoError(t, err)
			require.Len(t, got, len(tt.wantPlatforms))
			for i, b := range got {
				assert.Equal(t, tt.wantPlatforms[i], b.Platform)
				assert.Equal(t, tt.wantPath, b.Path)
				assert.Equal(t, want.GoVersion, b.BuildInfo.GoVersion)
				assert.Equal(t, want.Deps, b.BuildInfo.Deps)
			}
		})
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		push    func(t *testing.T, store *store) ocispec.Descriptor
		wantErr string
	}{
		"no entrypoint": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{platform: amd64, layers: []layer{uncompressed(regular("provider", []byte("x")))}})
			},
			wantErr: "the image sets no entrypoint or command",
		},
		"no binary": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{uncompressed(regular("manager", []byte("x")))}})
			},
			wantErr: "the image has no file /provider",
		},
		"not a Go binary": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{uncompressed(regular("provider", []byte("#!/bin/sh\n")))}})
			},
			wantErr: "/provider carries no Go build information",
		},
		"not an image": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return push(t, store, "application/vnd.cncf.helm.config.v1+json", []byte("{}"))
			},
			wantErr: "is a application/vnd.cncf.helm.config.v1+json, not an image",
		},
		"index of attestations": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushIndex(t, store, ocispec.MediaTypeImageIndex, ocispec.Descriptor{
					MediaType: ocispec.MediaTypeImageManifest,
					Digest:    digest.FromString("attestation"),
					Size:      1,
					Platform:  &ocispec.Platform{OS: "unknown", Architecture: "unknown"},
				})
			},
			wantErr: "lists no image",
		},
		"unsupported layer": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{
					{mediaType: "application/vnd.oci.image.layer.v1.tar+bzip2"},
				}})
			},
			wantErr: "unsupported layer media type application/vnd.oci.image.layer.v1.tar+bzip2",
		},
		"link loop": {
			push: func(t *testing.T, store *store) ocispec.Descriptor {
				return pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{
					uncompressed(symlink("provider", "manager"), symlink("manager", "/provider")),
				}})
			},
			wantErr: "more than 16 links lead from /provider",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newStore()
			require.NoError(t, store.Tag(t.Context(), tt.push(t, store), "1.0.0"))

			_, err := Read(t.Context(), store, "1.0.0")

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestFindFile checks that the binary is the file a container runtime
// would run, given the image's layers.
func TestFindFile(t *testing.T) {
	t.Parallel()

	file := func(name, data string) tarEntry { return regular(name, []byte(data)) }
	tests := map[string]struct {
		paths    []string
		layers   []layer
		wantPath string
		wantData string
		wantErr  string
	}{
		"higher layer wins": {
			paths:    []string{"/provider"},
			layers:   []layer{uncompressed(file("provider", "old")), gzipped(file("provider", "new"))},
			wantPath: "/provider", wantData: "new",
		},
		"later entry of a layer wins": {
			paths:    []string{"/provider"},
			layers:   []layer{uncompressed(file("provider", "first"), file("provider", "second"))},
			wantPath: "/provider", wantData: "second",
		},
		"PATH order beats layer order": {
			paths:    []string{"/usr/local/bin/manager", "/usr/bin/manager"},
			layers:   []layer{uncompressed(file("usr/local/bin/manager", "local")), uncompressed(file("usr/bin/manager", "system"))},
			wantPath: "/usr/local/bin/manager", wantData: "local",
		},
		"later PATH entry when earlier ones are absent": {
			paths:    []string{"/usr/local/bin/manager", "/usr/bin/manager"},
			layers:   []layer{uncompressed(file("usr/bin/manager", "system"))},
			wantPath: "/usr/bin/manager", wantData: "system",
		},
		"absolute symbolic link to a lower layer": {
			paths:    []string{"/provider"},
			layers:   []layer{uncompressed(file("app/manager", "app")), uncompressed(symlink("provider", "/app/manager"))},
			wantPath: "/app/manager", wantData: "app",
		},
		"hard link": {
			paths:    []string{"/provider"},
			layers:   []layer{uncompressed(file("app/manager", "app"), tarEntry{name: "provider", typeflag: tar.TypeLink, linkname: "app/manager"})},
			wantPath: "/app/manager", wantData: "app",
		},
		"whiteout": {
			paths:   []string{"/provider"},
			layers:  []layer{uncompressed(file("provider", "old")), uncompressed(file(".wh.provider", ""))},
			wantErr: "the image has no file /provider",
		},
		"whiteout of a directory": {
			paths:   []string{"/app/manager"},
			layers:  []layer{uncompressed(file("app/manager", "old")), uncompressed(file(".wh.app", ""))},
			wantErr: "the image has no file /app/manager",
		},
		"opaque directory": {
			paths:   []string{"/app/manager"},
			layers:  []layer{uncompressed(file("app/manager", "old")), uncompressed(dir("app/"), file("app/.wh..wh..opq", ""))},
			wantErr: "the image has no file /app/manager",
		},
		"opaque directory keeps its own layer's files": {
			paths:    []string{"/app/manager"},
			layers:   []layer{uncompressed(file("app/manager", "old")), uncompressed(dir("app/"), file("app/.wh..wh..opq", ""), file("app/manager", "new"))},
			wantPath: "/app/manager", wantData: "new",
		},
		"directory": {
			paths:   []string{"/provider"},
			layers:  []layer{uncompressed(dir("provider/"), file("provider/x", "x"))},
			wantErr: "the image has no file /provider",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newStore()
			var layers []ocispec.Descriptor
			for _, l := range tt.layers {
				layers = append(layers, push(t, store, l.mediaType, layerBlob(t, l)))
			}

			gotPath, gotData, err := findFile(t.Context(), store, layers, tt.paths)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, gotPath)
			assert.Equal(t, tt.wantData, string(gotData))
		})
	}
}

func TestEntrypoint(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config  ocispec.ImageConfig
		want    []string
		wantErr string
	}{
		"absolute entrypoint": {
			config: ocispec.ImageConfig{Entrypoint: []string{"/provider"}, Cmd: []string{"/other"}},
			want:   []string{"/provider"},
		},
		"command without an entrypoint": {
			config: ocispec.ImageConfig{Cmd: []string{"/bin/manager", "--flag"}},
			want:   []string{"/bin/manager"},
		},
		"relative to the working directory": {
			config: ocispec.ImageConfig{Entrypoint: []string{"./bin/manager"}, WorkingDir: "/workspace"},
			want:   []string{"/workspace/bin/manager"},
		},
		"looked up in the image's PATH": {
			config: ocispec.ImageConfig{Entrypoint: []string{"manager"}, Env: []string{"HOME=/root", "PATH=/app:relative:/usr/bin"}},
			want:   []string{"/app/manager", "/usr/bin/manager"},
		},
		"looked up in the default PATH": {
			config: ocispec.ImageConfig{Entrypoint: []string{"manager"}},
			want: []string{
				"/usr/local/sbin/manager", "/usr/local/bin/manager", "/usr/sbin/manager",
				"/usr/bin/manager", "/sbin/manager", "/bin/manager",
			},
		},
		"nothing to run": {
			config:  ocispec.ImageConfig{WorkingDir: "/"},
			wantErr: "the image sets no entrypoint or command",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := entrypoint(tt.config)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRemote(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	tests := map[string]struct {
		reference     string
		wantRef       string
		wantPlainHTTP bool
		wantErr       string
	}{
		"tag": {
			reference: "ghcr.io/openeverest/provider-x:1.2.3",
			wantRef:   "1.2.3",
		},
		"digest": {
			reference: "ghcr.io/openeverest/provider-x@sha256:" + strings.Repeat("a", 64),
			wantRef:   "sha256:" + strings.Repeat("a", 64),
		},
		"localhost": {
			reference:     "localhost:5000/provider-x:dev",
			wantRef:       "dev",
			wantPlainHTTP: true,
		},
		"loopback IPv4": {
			reference:     "127.0.0.1:5000/provider-x:dev",
			wantRef:       "dev",
			wantPlainHTTP: true,
		},
		"loopback IPv6": {
			reference:     "[::1]:5000/provider-x:dev",
			wantRef:       "dev",
			wantPlainHTTP: true,
		},
		"no tag": {
			reference: "ghcr.io/openeverest/provider-x",
			wantErr:   "names no tag or digest",
		},
		"no registry": {
			reference: "provider-x:1.2.3",
			wantErr:   `parsing image reference "provider-x:1.2.3"`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			target, ref, err := Remote(tt.reference)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, ref)
			repo, ok := target.(*remote.Repository)
			require.True(t, ok)
			assert.Equal(t, tt.wantPlainHTTP, repo.PlainHTTP)
		})
	}
}

// TestReadRemote reads an image from a registry on localhost, which serves
// the image of a memory store over plain HTTP.
func TestReadRemote(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", t.TempDir())

	bin, err := goBinary()
	require.NoError(t, err)
	store := newStore()
	index := pushIndex(t, store, ocispec.MediaTypeImageIndex,
		pushImage(t, store, imageSpec{platform: amd64, config: provider, layers: []layer{uncompressed(regular("provider", bin))}}),
	)
	require.NoError(t, store.Tag(t.Context(), index, "1.0.0"))
	server := httptest.NewServer(registry(t.Context(), store, "openeverest/provider-x"))
	t.Cleanup(server.Close)

	got, err := ReadRemote(t.Context(), strings.TrimPrefix(server.URL, "http://")+"/openeverest/provider-x:1.0.0")

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "linux/amd64", got[0].Platform)
	assert.Equal(t, "/provider", got[0].Path)
	want, ok := debug.ReadBuildInfo()
	require.True(t, ok)
	assert.Equal(t, want.Deps, got[0].BuildInfo.Deps)
}

// registry serves the read side of the OCI distribution API for the
// repository name from store.
func registry(ctx context.Context, store *store, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			return
		}
		rest, ok := strings.CutPrefix(r.URL.Path, "/v2/"+name+"/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, ref, _ := strings.Cut(rest, "/")
		desc, ok := store.find(ctx, ref)
		if !ok {
			http.NotFound(w, r)
			return
		}
		data, err := content.FetchAll(ctx, store, desc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", desc.MediaType)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Docker-Content-Digest", desc.Digest.String())
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(data)
	})
}
