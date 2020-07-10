package userdata

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

const (
	nodeUserData = `#!/bin/bash
/etc/eks/bootstrap.sh {{.ClusterName}}
`
)

// NodeInput defines the context to generate a node user data.
type NodeInput struct {
	ClusterName string
}

// NewNode returns the user data string to be used on a node instance.
func NewNode(input *NodeInput) ([]byte, error) {
	tm := template.New("Node")

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
