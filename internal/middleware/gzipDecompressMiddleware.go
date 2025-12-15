package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"skyrix/internal/logger"
	"strings"
)

type GzipDecompressMiddleware struct {
	logger logger.Interface
}

type readCloser struct {
	io.Reader
	close func() error
}

func (rc *readCloser) Close() error { return rc.close() }

func NewGzipDecompressMiddleware(logger logger.Interface) *GzipDecompressMiddleware {
	return &GzipDecompressMiddleware{
		logger: logger,
	}
}

func (m *GzipDecompressMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ce := r.Header.Get("Content-Encoding")
		if ce == "" || !strings.Contains(strings.ToLower(ce), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		limited := &io.LimitedReader{R: gr, N: 20 << 20}

		orig := r.Body
		r.Body = &readCloser{
			Reader: limited,
			close: func() error {
				_ = gr.Close()
				return orig.Close()
			},
		}
		toks := []string{}
		for _, t := range strings.Split(ce, ",") {
			t = strings.TrimSpace(strings.ToLower(t))
			if t != "" && t != "gzip" && t != "x-gzip" {
				toks = append(toks, t)
			}
		}
		if len(toks) == 0 {
			r.Header.Del("Content-Encoding")
		} else {
			r.Header.Set("Content-Encoding", strings.Join(toks, ", "))
		}
		r.ContentLength = -1
		next.ServeHTTP(w, r)
	})
}
