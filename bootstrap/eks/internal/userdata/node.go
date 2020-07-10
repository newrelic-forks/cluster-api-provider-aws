package userdata

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
	return generate("Node", nodeUserData, input)
}
