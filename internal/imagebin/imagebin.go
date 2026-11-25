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

// Package imagebin reads the Go build information of the binary a container
// image runs, on each of its platforms, from the image's registry: without a
// container runtime, and without running anything from the image.
package imagebin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/klauspost/compress/zstd"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// The media types of Docker's image manifest, version 2, schema 2, which
// registries serve for images Docker pushes.
const (
	dockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	dockerManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	dockerLayerGzip    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
)

// defaultPath is the PATH in which a container runtime looks up an
// entrypoint without a slash when the image sets none.
const defaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

const (
	// maxLinks bounds the links followed to the binary.
	maxLinks = 16

	// maxBinarySize bounds the binary read into memory.
	maxBinarySize = 1 << 30
)

// Binary is the binary an image runs on one platform.
type Binary struct {
	// Platform is the platform of the image, such as linux/amd64.
	Platform string

	// Path is the binary's path in the image.
	Path string

	// BuildInfo is the Go build information the binary embeds.
	BuildInfo *debug.BuildInfo
}

// ReadRemote returns the binary the image reference names runs on each of
// its platforms, read from its registry as Remote reaches it.
func ReadRemote(ctx context.Context, reference string) ([]Binary, error) {
	target, ref, err := Remote(reference)
	if err != nil {
		return nil, err
	}
	return Read(ctx, target, ref)
}

// Remote returns the repository of the image reference names, such as
// ghcr.io/openeverest/provider-x:1.2.3, which must name its registry, and
// the tag or digest that names the image in it. It reads credentials from
// the Docker configuration, as `docker login` writes them, and reaches a
// registry on the loopback interface, such as localhost:5000, over plain
// HTTP, as Docker does.
func Remote(reference string) (oras.ReadOnlyTarget, string, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, "", fmt.Errorf("parsing image reference %q: %w", reference, err)
	}
	ref := repo.Reference.Reference
	if ref == "" {
		return nil, "", fmt.Errorf("image reference %q names no tag or digest", reference)
	}
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("reading the Docker credentials: %w", err)
	}
	client := &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(store),
	}
	client.SetUserAgent("provider-sdk")
	repo.Client = client
	repo.PlainHTTP = loopback(repo.Reference.Registry)
	return repo, ref, nil
}

// loopback reports whether registry, a host with an optional port, is on
// the loopback interface.
func loopback(registry string) bool {
	host, _, err := net.SplitHostPort(registry)
	if err != nil {
		host = registry
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Read returns the binary the image reference names in target runs on each
// of its platforms: for an image index, the binary of every image it lists
// other than attestations, and for an image, its own.
func Read(ctx context.Context, target oras.ReadOnlyTarget, reference string) ([]Binary, error) {
	desc, err := target.Resolve(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", reference, err)
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageManifest, dockerManifest:
		b, err := readImage(ctx, target, desc)
		if err != nil {
			return nil, err
		}
		return []Binary{b}, nil
	case ocispec.MediaTypeImageIndex, dockerManifestList:
	default:
		return nil, fmt.Errorf("%s is a %s, not an image", reference, desc.MediaType)
	}

	data, err := content.FetchAll(ctx, target, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching the image index %s: %w", desc.Digest, err)
	}
	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decoding the image index %s: %w", desc.Digest, err)
	}
	var binaries []Binary
	for _, m := range index.Manifests {
		if !isImage(m) {
			continue
		}
		b, err := readImage(ctx, target, m)
		if err != nil {
			return nil, err
		}
		binaries = append(binaries, b)
	}
	if len(binaries) == 0 {
		return nil, fmt.Errorf("the image index %s lists no image", reference)
	}
	return binaries, nil
}

// isImage reports whether the entry m of an image index is an image, rather
// than an attestation, which BuildKit lists with the platform unknown/unknown.
func isImage(m ocispec.Descriptor) bool {
	if m.MediaType != ocispec.MediaTypeImageManifest && m.MediaType != dockerManifest {
		return false
	}
	if m.Annotations["vnd.docker.reference.type"] == "attestation-manifest" {
		return false
	}
	return m.Platform == nil || m.Platform.OS != "unknown"
}

// readImage returns the binary the image whose manifest is desc runs.
func readImage(ctx context.Context, target oras.ReadOnlyTarget, desc ocispec.Descriptor) (Binary, error) {
	data, err := content.FetchAll(ctx, target, desc)
	if err != nil {
		return Binary{}, fmt.Errorf("fetching the image manifest %s: %w", desc.Digest, err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Binary{}, fmt.Errorf("decoding the image manifest %s: %w", desc.Digest, err)
	}
	data, err = content.FetchAll(ctx, target, manifest.Config)
	if err != nil {
		return Binary{}, fmt.Errorf("fetching the image configuration %s: %w", manifest.Config.Digest, err)
	}
	var config ocispec.Image
	if err := json.Unmarshal(data, &config); err != nil {
		return Binary{}, fmt.Errorf("decoding the image configuration %s: %w", manifest.Config.Digest, err)
	}

	b := Binary{Platform: platform(config.Platform)}
	paths, err := entrypoint(config.Config)
	if err != nil {
		return Binary{}, fmt.Errorf("image %s (%s): %w", desc.Digest, b.Platform, err)
	}
	var bin []byte
	b.Path, bin, err = findFile(ctx, target, manifest.Layers, paths)
	if err != nil {
		return Binary{}, fmt.Errorf("image %s (%s): %w", desc.Digest, b.Platform, err)
	}
	if b.BuildInfo, err = buildinfo.Read(bytes.NewReader(bin)); err != nil {
		return Binary{}, fmt.Errorf("image %s (%s): %s carries no Go build information: %w", desc.Digest, b.Platform, b.Path, err)
	}
	return b, nil
}

func platform(p ocispec.Platform) string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// entrypoint returns the paths at which a container runtime looks for the
// binary the image runs, in its order, from the image's entrypoint, or its
// command when it sets no entrypoint.
func entrypoint(config ocispec.ImageConfig) ([]string, error) {
	args := slices.Concat(config.Entrypoint, config.Cmd)
	if len(args) == 0 || args[0] == "" {
		return nil, errors.New("the image sets no entrypoint or command")
	}
	name := args[0]
	switch {
	case path.IsAbs(name):
		return []string{path.Clean(name)}, nil
	case strings.Contains(name, "/"):
		return []string{path.Join("/", config.WorkingDir, name)}, nil
	}
	searchPath := defaultPath
	for _, env := range config.Env {
		if v, ok := strings.CutPrefix(env, "PATH="); ok {
			searchPath = v
		}
	}
	var paths []string
	for dir := range strings.SplitSeq(searchPath, ":") {
		if path.IsAbs(dir) {
			paths = append(paths, path.Join(dir, name))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("the image runs %s, but its PATH (%s) has no absolute directory", name, searchPath)
	}
	return paths, nil
}

// entry is what the filesystem of an image has at a path.
type entry struct {
	kind entryKind

	// data is the content of a file.
	data []byte

	// link is the absolute path a link points to.
	link string
}

type entryKind int

const (
	// unknown: no layer read so far has anything at the path.
	unknown entryKind = iota
	// absent: nothing the image can run, such as a directory, or a whiteout
	// that hides what lower layers have.
	absent
	file
	link
)

// findFile returns the first of paths that is a file in the filesystem the
// layers make, following links, and the file's content.
func findFile(ctx context.Context, target oras.ReadOnlyTarget, layers []ocispec.Descriptor, paths []string) (string, []byte, error) {
	start := strings.Join(paths, " or ")
	for range maxLinks {
		p, e, err := lookup(ctx, target, layers, paths)
		if err != nil {
			return "", nil, err
		}
		switch e.kind {
		case file:
			return p, e.data, nil
		case link:
			paths = []string{e.link}
		default:
			return "", nil, fmt.Errorf("the image has no file %s", strings.Join(paths, " or "))
		}
	}
	return "", nil, fmt.Errorf("more than %d links lead from %s", maxLinks, start)
}

// lookup returns the first of paths at which the filesystem the layers make
// has a file or a link, and what it has there. It reads the layers from the
// top, and stops once the paths before that one are known to be absent.
func lookup(ctx context.Context, target oras.ReadOnlyTarget, layers []ocispec.Descriptor, paths []string) (string, entry, error) {
	found := make([]entry, len(paths))
	for i := len(layers) - 1; i >= 0; i-- {
		layer, err := readLayer(ctx, target, layers[i], paths, found)
		if err != nil {
			return "", entry{}, fmt.Errorf("reading layer %s: %w", layers[i].Digest, err)
		}
		for j, e := range layer {
			if found[j].kind == unknown {
				found[j] = e
			}
		}
		if j, final := first(found); final {
			if j < 0 {
				return "", entry{kind: absent}, nil
			}
			return paths[j], found[j], nil
		}
	}
	for j, e := range found {
		if e.kind == file || e.kind == link {
			return paths[j], e, nil
		}
	}
	return "", entry{kind: absent}, nil
}

// first returns the index of the first of found that is a file or a link,
// or -1 if none is, and whether no entry before it is unknown, so that
// lower layers cannot change it.
func first(found []entry) (int, bool) {
	for j, e := range found {
		switch e.kind {
		case unknown:
			return -1, false
		case file, link:
			return j, true
		}
	}
	return -1, true
}

// readLayer returns what the layer desc has at each of paths that found,
// from the layers above it, does not know yet.
func readLayer(ctx context.Context, target oras.ReadOnlyTarget, desc ocispec.Descriptor, paths []string, found []entry) ([]entry, error) {
	rc, err := target.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	verified := content.NewVerifyReader(rc, desc)

	var r io.Reader
	switch desc.MediaType {
	case ocispec.MediaTypeImageLayer:
		r = verified
	case ocispec.MediaTypeImageLayerGzip, dockerLayerGzip:
		gz, err := gzip.NewReader(verified)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		r = gz
	case ocispec.MediaTypeImageLayerZstd:
		// One decoder decodes in this goroutine, which reads verified next.
		zr, err := zstd.NewReader(verified, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		r = zr
	default:
		return nil, fmt.Errorf("unsupported layer media type %s", desc.MediaType)
	}

	layer := make([]entry, len(paths))
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := path.Join("/", hdr.Name)
		dir, base := path.Split(name)
		if hidden, ok := strings.CutPrefix(base, ".wh."); ok {
			// A whiteout hides the path it names in lower layers, an
			// opaque whiteout what lower layers have in its directory. A
			// file of the same layer stays.
			opaque := hidden == ".wh..opq"
			hides := path.Join(dir, hidden)
			if opaque {
				hides = path.Clean(dir)
			}
			for j, p := range paths {
				if layer[j].kind == unknown && hiddenBy(p, hides, opaque) {
					layer[j] = entry{kind: absent}
				}
			}
			continue
		}
		for j, p := range paths {
			if p != name || found[j].kind != unknown {
				continue
			}
			switch hdr.Typeflag {
			case tar.TypeReg:
				if hdr.Size > maxBinarySize {
					return nil, fmt.Errorf("%s is larger than %d bytes", name, maxBinarySize)
				}
				data, err := io.ReadAll(tr)
				if err != nil {
					return nil, fmt.Errorf("reading %s: %w", name, err)
				}
				layer[j] = entry{kind: file, data: data}
			case tar.TypeSymlink:
				target := path.Join(dir, hdr.Linkname)
				if path.IsAbs(hdr.Linkname) {
					target = path.Clean(hdr.Linkname)
				}
				layer[j] = entry{kind: link, link: target}
			case tar.TypeLink:
				// A hard link names another file of the archive.
				layer[j] = entry{kind: link, link: path.Join("/", hdr.Linkname)}
			default:
				layer[j] = entry{kind: absent}
			}
		}
	}
	// Read what follows the archive, if anything, to check the layer's
	// digest.
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, verified); err != nil {
		return nil, err
	}
	if err := verified.Verify(); err != nil {
		return nil, err
	}
	return layer, nil
}

// hiddenBy reports whether a whiteout of hidden hides p, or, for an opaque
// whiteout of the directory hidden, whether p is inside it.
func hiddenBy(p, hidden string, opaque bool) bool {
	if !opaque && p == hidden {
		return true
	}
	return strings.HasPrefix(p, strings.TrimSuffix(hidden, "/")+"/")
}
