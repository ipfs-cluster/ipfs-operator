package main

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v2"
)

func DecodeObjs() (objs []map[string]any, err error) {
	dec := yaml.NewDecoder(os.Stdin)
	for {
		obj := make(map[string]any)
		if err := dec.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

func ModifyObj(obj map[string]any) error {
	var labels map[any]any
	meta := obj["metadata"].(map[any]any)
	_, ok := meta["labels"].(map[any]any)
	if !ok {
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
	if len(os.Args) > 1 {
		templateBase = os.Args[1]
	}

	objs, err := DecodeObjs()
	if err != nil {
		panic(err)
	}

	for _, obj := range objs {
		kind := obj["kind"].(string)
		meta := obj["metadata"].(map[any]any)
		name := meta["name"].(string)
		fileName := fmt.Sprintf("%s-%s.yaml", kind, name)
		baseName := templateBase
		if kind == "CustomResourceDefinition" {
			baseName = crdBase
		}

		if err := ModifyObj(obj); err != nil {
			fmt.Printf("error modifying obj %s: %v\n", fileName, err)
			os.Exit(1)
		}

		f, err := os.Create(fmt.Sprintf("%s/%s", baseName, fileName))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		enc := yaml.NewEncoder(f)
		if err := enc.Encode(obj); err != nil {
			fmt.Printf("error encoding obj %s: %v\n", fileName, err)
			os.Exit(1)
		}
	}
}
