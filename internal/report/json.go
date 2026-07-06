// Package report renders a model.Report as human text or stable JSON.
package report

import (
	"encoding/json"

	"github.com/wxggzz/ai-net-doctor/internal/model"
)

// JSON renders the report as indented, schema-versioned JSON.
func JSON(r model.Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
