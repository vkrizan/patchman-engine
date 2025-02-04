package base

import (
	"app/base/utils"
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"
)

const VMaaSAPIPrefix = "/api/v3"
const RBACApiPrefix = "/api/rbac/v1"

var Context context.Context
var CancelContext context.CancelFunc

func init() {
	Context, CancelContext = context.WithCancel(context.Background())
}

func HandleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		CancelContext()
		utils.Log().Info("SIGTERM/SIGINT handled")
	}()
}

func remove(r rune) rune {
	if r == 0 {
		return -1
	}
	return r
}

// Removes characters, which are not accepted by postgresql driver
// in parameter values
func RemoveInvalidChars(s string) string {
	return strings.Map(remove, s)
}

// TryExposeOnMetricsPort Expose app on required port if set
func TryExposeOnMetricsPort(app *gin.Engine) {
	metricsPort := utils.Cfg.MetricsPort
	if metricsPort == -1 {
		return // Do not expose extra metrics port if not set
	}
	err := utils.RunServer(Context, app, metricsPort)
	if err != nil {
		utils.Log("err", err.Error()).Error()
		panic(err)
	}
}
