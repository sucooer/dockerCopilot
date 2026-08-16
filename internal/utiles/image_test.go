package utiles

import (
	"encoding/binary"
	"testing"

	"github.com/docker/docker/api/types/image"
	MyType "github.com/onlyLTY/dockerCopilot/internal/types"
)

func TestSplitRepoTag(t *testing.T) {
	tests := []struct {
		in   string
		name string
		tag  string
	}{
		{in: "nginx:latest", name: "nginx", tag: "latest"},
		{in: "registry:5000/nginx:1.25", name: "registry:5000/nginx", tag: "1.25"},
		{in: "ubuntu", name: "ubuntu", tag: "latest"},
	}
	for _, tt := range tests {
		name, tag := splitRepoTag(tt.in)
		if name != tt.name || tag != tt.tag {
			t.Errorf("splitRepoTag(%q) = (%q, %q), want (%q, %q)", tt.in, name, tag, tt.name, tt.tag)
		}
	}
}

func TestSplitImageNameAndTag(t *testing.T) {
	images := []MyType.Image{
		{Summary: image.Summary{RepoTags: []string{"nginx:latest"}}},
		{Summary: image.Summary{RepoTags: []string{"registry:5000/nginx:1.25"}}},
		{Summary: image.Summary{RepoTags: []string{"ubuntu"}}},
		{Summary: image.Summary{RepoDigests: []string{"library/redis@sha256:abc"}}},
		{Summary: image.Summary{}},
	}
	got := splitImageNameAndTag(images)
	tests := []struct {
		idx  int
		name string
		tag  string
	}{
		{0, "nginx", "latest"},
		{1, "registry:5000/nginx", "1.25"},
		{2, "ubuntu", "latest"},
		{3, "library/redis", "None"},
		{4, "None", "None"},
	}
	for _, tt := range tests {
		if got[tt.idx].ImageName != tt.name || got[tt.idx].ImageTag != tt.tag {
			t.Errorf("case %d: got (%q, %q), want (%q, %q)", tt.idx, got[tt.idx].ImageName, got[tt.idx].ImageTag, tt.name, tt.tag)
		}
	}
}

func TestStripDockerStreamHeader(t *testing.T) {
	data := make([]byte, 0)
	data = append(data, appendFrame(1, []byte("hello"))...)
	data = append(data, appendFrame(2, []byte("err"))...)
	data = append(data, appendFrame(1, []byte("world"))...)
	out := stripDockerStreamHeader(data)
	if out != "helloerrworld" {
		t.Errorf("got %q", out)
	}
}

// TestStripDockerStreamHeaderFallback 验证非复用格式时原样返回
func TestStripDockerStreamHeaderFallback(t *testing.T) {
	raw := []byte("plain log line")
	if out := stripDockerStreamHeader(raw); out != "plain log line" {
		t.Errorf("got %q", out)
	}
}

func appendFrame(streamType byte, payload []byte) []byte {
	frame := make([]byte, 8+len(payload))
	frame[0] = streamType
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	copy(frame[8:], payload)
	return frame
}
