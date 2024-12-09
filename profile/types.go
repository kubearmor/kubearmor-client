// This code is directly taken from github.com/accuknox/auto-policy-discovery/src/types

package profile

// SpecPort Structure
type SpecPort struct {
	Port     string `json:"port,omitempty" yaml:"port,omitempty" bson:"port,omitempty"`
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty" bson:"protocol,omitempty"`
}
