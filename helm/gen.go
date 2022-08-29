package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

func DecodeObjs() ([]map[string]any, error) {
	dec := yaml.NewDecoder(os.Stdin)
	var objs = make([]map[string]any, 0)
	for {
		obj := make(map[string]any)
		if err := dec.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

func ModifyObj(obj map[string]any) error {
	var labels, meta map[any]any
	var ok bool
	if meta, ok = obj["metadata"].(map[any]any); !ok {
		return fmt.Errorf("no metadata could be found")
	}
	if _, ok = meta["labels"].(map[any]any); !ok {
		labels = make(map[any]any)
		meta["labels"] = labels
	}
	// labels["app.kubernetes.io/managed-by"] = "{{ $.Release.Service }}"
	// labels["app.kubernetes.io/instance"] = "{{ $.Release.Name }}"
	// labels["app.kubernetes.io/version"] = "{{ $.Chart.AppVersion}}"
	// labels["helm.sh/chart"] = "{{ $.Chart.Name }}-{{ $.Chart.Version }}"

	var annotations map[any]any
	_, ok = meta["annotations"].(map[any]any)
	if !ok {
		annotations = make(map[any]any)
		meta["annotations"] = annotations
	}
	// annotations["meta.helm.sh/release-name"] = "{{ $.Release.Name }}"
	// annotations["meta.helm.sh/release-namespace"] = "{{ $.Release.Namespace }}"

	return nil
}

func main() {
	templateBase := "./helm/ipfs-operator/templates"
	crdBase := "./helm/ipfs-operator/crds"
	var err error
	if len(os.Args) > 1 {
		templateBase = os.Args[1]
	}

	objs, err := DecodeObjs()
	if err != nil {
		panic(err)
	}

	for _, obj := range objs {
		var f *os.File
		// ignore if these values are not set
		var ok bool
		var kind, name string
		var meta map[any]any
		if kind, ok = obj["kind"].(string); !ok {
			log.Printf("no kind found in %v, skipping\n", obj)
			continue
		}
		if meta, ok = obj["metadata"].(map[any]any); !ok {
			log.Printf("no metadata could be found\n")
			continue
		}
		if name, ok = meta["name"].(string); !ok {
			log.Printf("name not found")
			continue
		}

		fileName := fmt.Sprintf("%s-%s.yaml", kind, name)
		baseName := templateBase
		if kind == "CustomResourceDefinition" {
			baseName = crdBase
		}

		if err = ModifyObj(obj); err != nil {
			log.Printf("error modifying obj %s: %v\n", fileName, err)
			os.Exit(1)
		}

		if f, err = os.Create(fmt.Sprintf("%s/%s", baseName, fileName)); err != nil {
			log.Printf("error creating file %s: %v\n", fileName, err)
			os.Exit(1)
		}
		defer f.Close()
		enc := yaml.NewEncoder(f)
		if err = enc.Encode(obj); err != nil {
			log.Printf("error encoding obj %s: %v\n", fileName, err)
			return
		}
	}
}
