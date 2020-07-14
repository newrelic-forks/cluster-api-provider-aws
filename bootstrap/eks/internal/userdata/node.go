/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package userdata

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

const (
	nodeUserData = `#!/bin/bash
/etc/eks/bootstrap.sh {{.ClusterName}} {{- template "args" .KubeletExtraArgs }}
`
)

// NodeInput defines the context to generate a node user data.
type NodeInput struct {
	ClusterName      string
	KubeletExtraArgs map[string]string
}

// NewNode returns the user data string to be used on a node instance.
func NewNode(input *NodeInput) ([]byte, error) {
	tm := template.New("Node")

	if _, err := tm.Parse(argsTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse args template")
	}

	if _, err := tm.Parse(kubeletArgsTemplate); err != nil {
		return nil, errors.Wrap(err, "failed to parse kubeletExtraArgs template")
	}

	t, err := tm.Parse(nodeUserData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse Node template")
	}

	var out bytes.Buffer
	if err := t.Execute(&out, input); err != nil {
		return nil, errors.Wrapf(err, "failed to generate Node template")
	}

	return out.Bytes(), nil
}
