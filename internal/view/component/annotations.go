package component

import "encoding/json"

// Annotations is a component representing key/value based annotations
type Annotations struct {
	Metadata Metadata          `json:"metadata"`
	Config   AnnotationsConfig `json:"config"`
}

// AnnotationsConfig is the contents of Annotations
type AnnotationsConfig struct {
	Annotations map[string]string `json:"annotations"`
}

// NewAnnotations creates a annotations component
func NewAnnotations(annotations map[string]string) *Annotations {
	return &Annotations{
		Metadata: Metadata{
			Type: "annotations",
		},
		Config: AnnotationsConfig{
			Annotations: annotations,
		},
	}
}

// GetMetadata accesses the components metadata. Implements ViewComponent.
func (t *Annotations) GetMetadata() Metadata {
	return t.Metadata
}

// IsEmpty specifies whether the component is considered empty. Implements ViewComponent.
func (t *Annotations) IsEmpty() bool {
	return len(t.Config.Annotations) == 0
}

type annotationsMarshal Annotations

// MarshalJSON implements json.Marshaler.
func (t *Annotations) MarshalJSON() ([]byte, error) {
	m := annotationsMarshal(*t)
	m.Metadata.Type = "annotations"
	m.Metadata.Title = t.Metadata.Title
	return json.Marshal(&m)
}
