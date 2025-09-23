package watcher

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

func (w *WatcherAPIClient) PrintCheckResult(params CheckQueryParams) error {
	resp := w.GetCheck(params)

	if resp.IsErr() {
		return errors.Wrap(resp.UnwrapErr(), "failed call to GetCheck")
	} else {
		jsonBytes, err := json.MarshalIndent(resp.Unwrap(), "", "\t")
		if err != nil {
			return errors.Wrap(err, "failed call to MarshalIndent")
		}
		fmt.Printf("Response:\n%s\n", string(jsonBytes))
	}
	return nil
}

func (w *WatcherAPIClient) PrintLatencyResult(params LatencyQueryParams) error {
	resp := w.GetLatency(params)
	if resp.IsErr() {
		return errors.Wrap(resp.UnwrapErr(), "failed call to GetLatency")
	} else {
		jsonBytes, err := json.MarshalIndent(resp.Unwrap(), "", "\t")
		if err != nil {
			return errors.Wrap(err, "failed call to MarshalIndent")
		}
		fmt.Printf("Response:\n%s\n", string(jsonBytes))
	}
	return nil
}
