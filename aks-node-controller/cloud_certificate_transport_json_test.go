package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudOperationCertificateGroupJSON(t *testing.T) {
	payloads := map[string][]byte{
		"root.crt":   cloudTestCertificate(t),
		"second.crt": cloudTestCertificate(t),
	}
	testCases := []struct {
		name      string
		response  string
		wantNames []string
		wantErr   string
	}{
		{
			name:      "one space after colon",
			response:  `{"OperationsInfo":[{"ResouceFileName": "root.crt"}]}`,
			wantNames: []string{"root.crt"},
		},
		{
			name:      "compact JSON",
			response:  `{"OperationsInfo":[{"ResouceFileName":"root.crt"}]}`,
			wantNames: []string{"root.crt"},
		},
		{
			name: "indented JSON",
			response: `{
  "OperationsInfo": [
    {
      "ResouceFileName": "root.crt"
    }
  ]
}`,
			wantNames: []string{"root.crt"},
		},
		{
			name:      "tabs CRLF and whitespace around colon",
			response:  "\r\n{\r\n\t\"OperationsInfo\"\t:\t[\r\n\t{\"ResouceFileName\"\r\n:\t \"root.crt\"}\r\n]\r\n}\t\r\n",
			wantNames: []string{"root.crt"},
		},
		{
			name:      "JSON escaped filename",
			response:  `{"OperationsInfo":[{"ResouceFileName":"\u0072oot.crt"}]}`,
			wantNames: []string{"root.crt"},
		},
		{
			name: "ignore filenames at other levels",
			response: `{
  "ResouceFileName": "wrong.crt",
  "Metadata": {"ResouceFileName": "nested.crt"},
  "OperationsInfo": [
    {"ResouceFileName": "root.crt", "Metadata": {"ResouceFileName": "nested-entry.crt"}}
  ]
}`,
			wantNames: []string{"root.crt"},
		},
		{
			name: "skip non-string and missing names",
			response: `{"OperationsInfo":[
  {"ResouceFileName":42},
  {"ResouceFileName":null},
  {"OtherField":"ignored"},
  {"ResouceFileName":"root.crt"}
]}`,
			wantNames: []string{"root.crt"},
		},
		{
			name:      "multiple resources",
			response:  `{"OperationsInfo":[{"ResouceFileName":"root.crt"},{"ResouceFileName":"second.crt"}]}`,
			wantNames: []string{"root.crt", "second.crt"},
		},
		{
			name:     "truncated JSON containing a filename",
			response: `{"OperationsInfo":[{"ResouceFileName": "root.crt"}]`,
			wantErr:  "decode Azure cloud",
		},
		{
			name:     "filename only at the wrong level",
			response: `{"ResouceFileName": "root.crt"}`,
			wantErr:  "contains no certificate filenames",
		},
		{
			name:     "top-level array",
			response: `[{"ResouceFileName": "root.crt"}]`,
			wantErr:  "decode Azure cloud",
		},
		{
			name:     "OperationsInfo is not an array",
			response: `{"OperationsInfo":{"ResouceFileName": "root.crt"}}`,
			wantErr:  "decode Azure cloud",
		},
		{
			name:     "corrected spelling is not the protocol field",
			response: `{"OperationsInfo":[{"ResourceFileName": "root.crt"}]}`,
			wantErr:  "contains no certificate filenames",
		},
		{
			name:     "null OperationsInfo",
			response: `{"OperationsInfo":null}`,
			wantErr:  "contains no certificate filenames",
		},
		{
			name:     "empty OperationsInfo",
			response: `{"OperationsInfo":[]}`,
			wantErr:  "contains no certificate filenames",
		},
		{
			name:     "no string filenames",
			response: `{"OperationsInfo":[{"ResouceFileName":42},{"ResouceFileName":null}]}`,
			wantErr:  "contains no certificate filenames",
		},
	}
	for _, kind := range []string{"operationrequestsroot", "operationrequestsintermediate"} {
		for _, tc := range testCases {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				c := newTestCloudInitializer(t, cloudTestUbuntu)
				manifestURI := "/machine?comp=acmspackage&type=" + kind + "&ext=json"
				var manifests, downloads atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.Method + " " + r.RequestURI {
					case http.MethodGet + " " + manifestURI:
						manifests.Add(1)
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, tc.response)
					case http.MethodGet + " /machine?comp=acmspackage&type=root&ext=crt":
						downloads.Add(1)
						_, _ = w.Write(payloads["root.crt"])
					case http.MethodGet + " /machine?comp=acmspackage&type=second&ext=crt":
						downloads.Add(1)
						_, _ = w.Write(payloads["second.crt"])
					default:
						t.Errorf("unexpected certificate request: %s %s", r.Method, r.RequestURI)
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer server.Close()
				c.wireEndpoint = server.URL

				certificates, err := c.operationCertificateGroup(t.Context(), kind)
				require.EqualValues(t, 1, manifests.Load())
				require.EqualValues(t, len(tc.wantNames), downloads.Load())
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					require.Nil(t, certificates)
					_, statErr := os.Stat(c.path("/root/AzureCACertificates"))
					require.ErrorIs(t, statErr, os.ErrNotExist)
					return
				}
				require.NoError(t, err)
				want := make(map[string][]byte, len(tc.wantNames))
				for _, name := range tc.wantNames {
					want[name] = payloads[name]
					cached, readErr := os.ReadFile(c.path("/root/AzureCACertificates/" + name))
					require.NoError(t, readErr)
					require.Equal(t, payloads[name], cached)
				}
				require.Equal(t, want, certificates)
				cached, readErr := os.ReadDir(c.path("/root/AzureCACertificates"))
				require.NoError(t, readErr)
				require.Len(t, cached, len(tc.wantNames))
			})
		}
	}
}
