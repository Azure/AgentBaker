package agent

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/Azure/agentbaker/parts"
)

func TestGzipWireCompatibility(t *testing.T) {
	for name, input := range map[string][]byte{
		"empty":  {},
		"binary": {0, 1, 127, 128, 255, '\r', '\n'},
		"text":   []byte("certificate data\n\"quoted\" \\ path\n"),
		"large":  bytes.Repeat([]byte("cloud-init write_files and CA refresh\n"), 10000),
	} {
		t.Run(name, func(t *testing.T) {
			encoded := getGzippedBufferFromBytes(input)
			if !bytes.Equal(encoded, getGzippedBufferFromBytes(input)) {
				t.Fatal("gzip encoding must be deterministic")
			}
			reader, err := gzip.NewReader(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("standard gzip reader rejected payload: %v", err)
			}
			decoded, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("standard gzip reader failed: %v", err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(input, decoded) {
				t.Fatal("decompressed artifact bytes changed")
			}
		})
	}
}

func BenchmarkScriptCompression(b *testing.B) {
	for _, path := range []string{initAKSCloudScript, kubernetesCSEConfig} {
		source, err := parts.Templates.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		source = removeComments(source)
		b.Run(path, func(b *testing.B) {
			b.Run("standard", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(source)))
				for b.Loop() {
					var buf bytes.Buffer
					writer, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
					if err != nil {
						b.Fatal(err)
					}
					if _, err := writer.Write(source); err != nil {
						b.Fatal(err)
					}
					if err := writer.Close(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("production", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(source)))
				for b.Loop() {
					getGzippedBufferFromBytes(source)
				}
			})
		})
	}
}
