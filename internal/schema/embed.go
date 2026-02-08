package schema

import (
	_ "embed"
	"encoding/json"

	"gopkg.in/yaml.v3"
)

//go:embed timeline.openapi.yaml
var timelineOpenAPI []byte

//go:embed notifications.openapi.yaml
var notificationsOpenAPI []byte

func TimelineOpenAPIJSON() ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(timelineOpenAPI, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func NotificationsOpenAPIJSON() ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(notificationsOpenAPI, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
