/*
Copyright 2023.

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

package v1alpha1

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apiextensions-apiserver/pkg/registry/customresource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	rts "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestArbitraryRuleDataEncode(t *testing.T) {
	ecp := EnterpriseContractPolicy{
		Spec: EnterpriseContractPolicySpec{
			Sources: []Source{
				{
					RuleData: &v1.JSON{Raw: []byte(`{"my":"data","here":14}`)},
				},
			},
		},
	}

	out := strings.Builder{}
	err := unstructured.UnstructuredJSONScheme.Encode(&ecp, &out)
	if err != nil {
		t.Fatalf("Unexpected error when encoding: %s", err)
	}

	expected := `{"metadata":{"creationTimestamp":null},"spec":{"sources":[{"ruleData":{"my":"data","here":14}}]},"status":{}}` + "\n"
	got := out.String()
	if got != expected {
		t.Errorf("Expecting encoded to be: %s, but it was %s", expected, got)
	}
}

func TestArbitraryRuleDataDecode(t *testing.T) {
	ecp := EnterpriseContractPolicy{}
	_, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(`{"apiVersion":"appstudio.redhat.com/v1alpha1","kind":"EnterpriseContractPolicy","spec":{"sources":[{"ruleData":{"my":"data","here":14}}]}}`), nil, &ecp)
	if err != nil {
		t.Fatalf("Unexpected error when encoding: %s", err)
	}

	expected := `{"my":"data","here":14}`
	got := string(ecp.Spec.Sources[0].RuleData.Raw)
	if got != expected {
		t.Errorf("Expecting decoded to be: %s, but it was %s", expected, got)
	}
}

func TestMultiplevolatileConfigWithSameValue(t *testing.T) {
	data := []byte(`{
		"apiVersion":"appstudio.redhat.com/v1alpha1",
		"kind":"EnterpriseContractPolicy",
		"metadata": {
			"name": "test"
		},
		"spec":{
			"sources": [
				{
					"volatileConfig": {
						"exclude": [
							{
								"value": "a",
								"imageRef": "sha256:cfe1335814d92eabecfe9802f13298539caa7bbd0a13b61f320dc45bdded473d"
							},
							{
								"value": "a",
								"imageRef": "sha256:1f88f9fb4543eadf97afcbd417c258fdf1a02dd000a36e39e7e4649d1b083b4e"
							},
							{
								"value": "a",
								"imageRef": "sha256:c294f4f54f5a16b2c2a1dae988bc45972760e2a7f6c68eb9eb20329bfe126062"
							}
						]
					}
				}
			]
		}
	}`)

	ecp := EnterpriseContractPolicy{}
	_, _, err := unstructured.UnstructuredJSONScheme.Decode(data, nil, &ecp)
	if err != nil {
		t.Fatalf("unexpected error when encoding: %s", err)
	}

	if expected, got := 3, len(ecp.Spec.Sources[0].VolatileConfig.Exclude); expected != got {
		t.Errorf("expected %d excludes in volatile config, got: %d", expected, got)
	}

	crd := v1.CustomResourceDefinition{}
	bytes, _ := os.ReadFile("../config/appstudio.redhat.com_enterprisecontractpolicies.yaml")
	if err := yaml.Unmarshal(bytes, &crd); err != nil {
		t.Fatalf("unexpected error when decoding schema: %s", err)
	}

	crdv := apiextensions.CustomResourceValidation{}
	if err := v1.Convert_v1_CustomResourceValidation_To_apiextensions_CustomResourceValidation(crd.Spec.Versions[0].Schema, &crdv, nil); err != nil {
		t.Fatalf("failed in CRD validation conversion: %s", err)
	}

	s, err := schema.NewStructural(crdv.OpenAPIV3Schema)
	if err != nil {
		t.Fatalf("unexpected error when creating structural: %s", err)
	}

	v := validation.NewSchemaValidatorFromOpenAPI(s.ToKubeOpenAPI())

	r := v.Validate(ecp)

	if !r.IsValid() {
		t.Errorf("failed schema validation with: %v", r)
	}

	obj := unstructured.Unstructured{}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unexpected error when unmarshalling: %s", err)
	}

	gvk := rts.GroupVersionKind{Group: "appstudio.redhat.com", Version: "v1alpha1", Kind: "EnterpriseContractPolicy"}
	errs := customresource.NewStrategy(nil, false, gvk, v, nil, s, nil, nil).Validate(context.Background(), &obj)

	if len(errs) > 0 {
		t.Errorf("did not expect validation errors: %v", errs)
	}
}
