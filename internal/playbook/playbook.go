package playbook

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Step is one connector action inside a playbook.
type Step struct {
	Action  string `yaml:"action" json:"action"`
	Payload string `yaml:"payload" json:"payload"`
}

// Doc is the YAML document stored on a playbook.
type Doc struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

// Parse checks that the document has a name and at least one action.
func Parse(body string) (Doc, error) {
	var doc Doc
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		return Doc{}, fmt.Errorf("invalid playbook yaml")
	}
	if doc.Name == "" {
		return Doc{}, fmt.Errorf("name required")
	}
	if len(doc.Steps) == 0 {
		return Doc{}, fmt.Errorf("steps required")
	}
	for i, step := range doc.Steps {
		if step.Action == "" {
			return Doc{}, fmt.Errorf("step %d needs an action", i+1)
		}
		if doc.Steps[i].Payload == "" {
			doc.Steps[i].Payload = "{}"
		}
	}
	return doc, nil
}

// Render writes the document back to YAML.
func Render(doc Doc) (string, error) {
	raw, err := yaml.Marshal(&doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
