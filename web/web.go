package web

import (
	"net/http"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Lean struct {
	http.Handler
}

var (
	responseDurations = promauto.NewSummaryVec(prometheus.SummaryOpts{
		Name: "leanweb_response_duration",
		Help: "HTTP Response Duration",
	}, []string{"method", "path"})

	responseStatusCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "leanweb_response_status_count",
		Help: "HTTP Status per response",
	}, []string{"status", "method", "path"})
)

var handlerRegexp = regexp.MustCompile(`^@([A-Z]+).js$`)
