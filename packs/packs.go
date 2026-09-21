// Package packs holds starter templates and one dashboard for each vertical.
package packs

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed manufacturing.yaml data-center.yaml hospital.yaml cold-chain.yaml utilities.yaml
var files embed.FS

//go:embed connectors/servicenow.yaml connectors/maximo.yaml connectors/sap.yaml
var connectorFiles embed.FS

var Names = []string{"manufacturing", "data-center", "hospital", "cold-chain", "utilities"}

type Template struct {
	Name         string   `yaml:"name"`
	Kind         string   `yaml:"kind"`
	Capabilities []string `yaml:"capabilities"`
}

type Dashboard struct {
	Name         string   `yaml:"name"`
	Capabilities []string `yaml:"capabilities"`
}

type Automation struct {
	Name       string  `yaml:"name"`
	Capability string  `yaml:"capability"`
	Operator   string  `yaml:"operator"`
	Threshold  float64 `yaml:"threshold"`
	Action     string  `yaml:"action"`
}

type File struct {
	Name        string       `yaml:"name"`
	Templates   []Template   `yaml:"templates"`
	Dashboard   Dashboard    `yaml:"dashboard"`
	Automations []Automation `yaml:"automations"`
	WorkOrder   string       `yaml:"work_order"`
}

type ConnectorFile struct {
	Name   string         `yaml:"name" json:"name"`
	Kind   string         `yaml:"kind" json:"kind"`
	Action string         `yaml:"action" json:"action"`
	Body   map[string]any `yaml:"body" json:"body"`
}

func Read(name string) ([]byte, error) {
	return files.ReadFile(name + ".yaml")
}

func Parse(raw []byte) (File, error) {
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return f, err
	}
	if f.Name == "" || len(f.Templates) == 0 || f.Dashboard.Name == "" || len(f.Dashboard.Capabilities) == 0 {
		return f, fmt.Errorf("pack needs a name, a template, and a dashboard")
	}
	if len(f.Automations) < 2 || f.WorkOrder == "" {
		return f, fmt.Errorf("pack needs two automations and a work order")
	}
	return f, nil
}

func Connectors() []ConnectorFile {
	var out []ConnectorFile
	for _, name := range []string{"servicenow", "maximo", "sap"} {
		f, err := Connector(name)
		if err == nil {
			out = append(out, f)
		}
	}
	if out == nil {
		out = []ConnectorFile{}
	}
	return out
}

func Connector(name string) (ConnectorFile, error) {
	raw, err := connectorFiles.ReadFile("connectors/" + name + ".yaml")
	if err != nil {
		return ConnectorFile{}, err
	}
	var f ConnectorFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return ConnectorFile{}, err
	}
	if f.Kind == "" || f.Action == "" {
		return ConnectorFile{}, fmt.Errorf("connector pack incomplete")
	}
	return f, nil
}
