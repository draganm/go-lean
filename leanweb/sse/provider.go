package sse

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dop251/goja"
	"github.com/draganm/go-lean/common/globals"
)

type serverEvent struct {
	ID    string
	Event string
	Data  string
}

func NewProvider() globals.RequestGlobalsProvider {
	return func(handlerPath string, vm *goja.Runtime, w http.ResponseWriter, r *http.Request) (any, error) {
		return func(nextEvent func() (*serverEvent, error)) error {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			writeMessage := func(m *serverEvent) error {
				if len(m.ID) > 0 {
					_, err := fmt.Fprintf(w, "id: %s\n", strings.Replace(m.ID, "\n", "", -1))
					if err != nil {
						return err
					}
				}
				if len(m.Event) > 0 {
					_, err := fmt.Fprintf(w, "event: %s\n", strings.Replace(m.Event, "\n", "", -1))
					if err != nil {
						return err
					}
				}
				if len(m.Data) > 0 {
					lines := strings.Split(m.Data, "\n")
					for _, line := range lines {
						_, err := fmt.Fprintf(w, "data: %s\n", line)
						if err != nil {
							return err
						}
					}
				}
				_, err := w.Write([]byte("\n"))
				if err != nil {
					return err
				}

				rc := http.NewResponseController(w)
				rc.Flush()

				return nil

			}

			for {
				evt, err := nextEvent()
				if err != nil {
					return err
				}

				if evt == nil {
					return nil
				}

				err = writeMessage(evt)
				if err != nil {
					return err
				}

			}
		}, nil
	}

}
