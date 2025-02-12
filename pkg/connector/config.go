// mautrix-linkedin - A Matrix-LinkedIn puppeting bridge.
// Copyright (C) 2025 Sumner Evans
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package connector

import (
	_ "embed"
	"strings"
	"text/template"

	up "go.mau.fi/util/configupgrade"
	"gopkg.in/yaml.v3"
)

//go:embed example-config.yaml
var ExampleConfig string

type Config struct {
	DisplaynameTemplate string             `yaml:"displayname_template"`
	displaynameTemplate *template.Template `yaml:"-"`
}

type umConfig Config

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	err := node.Decode((*umConfig)(c))
	if err != nil {
		return err
	}
	return c.PostProcess()
}

func (c *Config) PostProcess() (err error) {
	c.displaynameTemplate, err = template.New("displayname").Parse(c.DisplaynameTemplate)
	return
}

func upgradeConfig(helper up.Helper) {
	helper.Copy(up.Str, "displayname_template")
}

func (lc *LinkedInConnector) GetConfig() (string, any, up.Upgrader) {
	return ExampleConfig, &lc.Config, up.SimpleUpgrader(upgradeConfig)
}

type DisplaynameParams struct {
	FirstName string
	LastName  string
}

func (c *Config) FormatDisplayname(params DisplaynameParams) string {
	var buffer strings.Builder
	_ = c.displaynameTemplate.Execute(&buffer, params)
	return buffer.String()
}
