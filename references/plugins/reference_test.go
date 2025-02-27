/*
Copyright 2021 The KubeVela Authors.

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

package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/oam-dev/kubevela/apis/types"
)

var RefTestDir = filepath.Join(TestDir, "ref")

func TestCreateRefTestDir(t *testing.T) {
	if _, err := os.Stat(RefTestDir); err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(RefTestDir, 0750)
		assert.NoError(t, err)
	}
}

func TestCreateMarkdown(t *testing.T) {
	workloadName := "workload1"
	traitName := "trait1"
	scopeName := "scope1"
	workloadName2 := "workload2"

	workloadCueTemplate := `
parameter: {
	// +usage=Which image would you like to use for your service
	// +short=i
	image: string
}
`
	traitCueTemplate := `
parameter: {
	replicas: int
}
`

	configuration := `
resource "alicloud_oss_bucket" "bucket-acl" {
  bucket = var.bucket
  acl = var.acl
}

output "BUCKET_NAME" {
  value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
}

variable "bucket" {
  description = "OSS bucket name"
  default = "vela-website"
  type = string
}

variable "acl" {
  description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
  default = "private"
  type = string
}
`

	cases := map[string]struct {
		reason       string
		capabilities []types.Capability
		want         error
	}{
		"WorkloadTypeAndTraitCapability": {
			reason: "valid capabilities",
			capabilities: []types.Capability{
				{
					Name:        workloadName,
					Type:        types.TypeWorkload,
					CueTemplate: workloadCueTemplate,
					Category:    types.CUECategory,
				},
				{
					Name:        traitName,
					Type:        types.TypeTrait,
					CueTemplate: traitCueTemplate,
					Category:    types.CUECategory,
				},
				{
					Name:                   workloadName2,
					TerraformConfiguration: configuration,
					Type:                   types.TypeWorkload,
					Category:               types.TerraformCategory,
				},
			},
			want: nil,
		},
		"ScopeTypeCapability": {
			reason: "invalid capabilities",
			capabilities: []types.Capability{
				{
					Name: scopeName,
					Type: types.TypeScope,
				},
			},
			want: fmt.Errorf("the type of the capability is not right"),
		},
	}
	ref := &MarkdownReference{}
	ctx := context.Background()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ref.CreateMarkdown(ctx, tc.capabilities, RefTestDir, ReferenceSourcePath)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCreateMakrdown(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}

}

func TestPrepareParameterTable(t *testing.T) {
	ref := MarkdownReference{}
	tableName := "hello"
	parameterList := []ReferenceParameter{
		{
			PrintableType: "string",
		},
	}
	parameterName := "cpu"
	parameterList[0].Name = parameterName
	parameterList[0].Required = true
	refContent := ref.prepareParameter(tableName, parameterList, types.CUECategory)
	assert.Contains(t, refContent, parameterName)
	assert.Contains(t, refContent, "cpu")
}

func TestDeleteRefTestDir(t *testing.T) {
	if _, err := os.Stat(RefTestDir); err == nil {
		err := os.RemoveAll(RefTestDir)
		assert.NoError(t, err)
	}
}

func TestWalkParameterSchema(t *testing.T) {
	testcases := []struct {
		data       string
		ExpectRefs map[string]map[string]ReferenceParameter
	}{
		{
			data: `{
    "properties": {
        "cmd": {
            "description": "Commands to run in the container", 
            "items": {
                "type": "string"
            }, 
            "title": "cmd", 
            "type": "array"
        }, 
        "image": {
            "description": "Which image would you like to use for your service", 
            "title": "image", 
            "type": "string"
        }
    }, 
    "required": [
        "image"
    ], 
    "type": "object"
}`,
			ExpectRefs: map[string]map[string]ReferenceParameter{
				"# Properties": {
					"cmd": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "cmd",
							Usage:    "Commands to run in the container",
							JSONType: "array",
						},
						PrintableType: "array",
					},
					"image": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "image",
							Required: true,
							Usage:    "Which image would you like to use for your service",
							JSONType: "string",
						},
						PrintableType: "string",
					},
				},
			},
		},
		{
			data: `{
    "properties": { 
        "obj": {
            "properties": {
                "f0": {
                    "default": "v0", 
                    "type": "string"
                }, 
                "f1": {
                    "default": "v1", 
                    "type": "string"
                }, 
                "f2": {
                    "default": "v2", 
                    "type": "string"
                }
            }, 
            "type": "object"
        },
    }, 
    "type": "object"
}`,
			ExpectRefs: map[string]map[string]ReferenceParameter{
				"# Properties": {
					"obj": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "obj",
							JSONType: "object",
						},
						PrintableType: "[obj](#obj)",
					},
				},
				"## obj": {
					"f0": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "f0",
							Default:  "v0",
							JSONType: "string",
						},
						PrintableType: "string",
					},
					"f1": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "f1",
							Default:  "v1",
							JSONType: "string",
						},
						PrintableType: "string",
					},
					"f2": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "f2",
							Default:  "v2",
							JSONType: "string",
						},
						PrintableType: "string",
					},
				},
			},
		},
		{
			data: `{
    "properties": {
        "obj": {
            "properties": {
                "f0": {
                    "default": "v0", 
                    "type": "string"
                }, 
                "f1": {
                    "default": "v1", 
                    "type": "object", 
                    "properties": {
                        "g0": {
                            "default": "v2", 
                            "type": "string"
                        }
                    }
                }
            }, 
            "type": "object"
        }
    }, 
    "type": "object"
}`,
			ExpectRefs: map[string]map[string]ReferenceParameter{
				"# Properties": {
					"obj": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "obj",
							JSONType: "object",
						},
						PrintableType: "[obj](#obj)",
					},
				},
				"## obj": {
					"f0": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "f0",
							Default:  "v0",
							JSONType: "string",
						},
						PrintableType: "string",
					},
					"f1": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "f1",
							Default:  "v1",
							JSONType: "object",
						},
						PrintableType: "[f1](#f1)",
					},
				},
				"### f1": {
					"g0": ReferenceParameter{
						Parameter: types.Parameter{
							Name:     "g0",
							Default:  "v2",
							JSONType: "string",
						},
						PrintableType: "string",
					},
				},
			},
		},
	}
	for _, cases := range testcases {
		commonRefs = make([]CommonReference, 0)
		parameterJSON := fmt.Sprintf(BaseOpenAPIV3Template, cases.data)
		swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(json.RawMessage(parameterJSON))
		assert.Equal(t, nil, err)
		parameters := swagger.Components.Schemas["parameter"].Value
		WalkParameterSchema(parameters, "Properties", 0)
		refs := make(map[string]map[string]ReferenceParameter)
		for _, items := range commonRefs {
			refs[items.Name] = make(map[string]ReferenceParameter)
			for _, item := range items.Parameters {
				refs[items.Name][item.Name] = item
			}
		}
		assert.Equal(t, true, reflect.DeepEqual(cases.ExpectRefs, refs))
	}
}

func TestGenerateTerraformCapabilityProperties(t *testing.T) {
	ref := &ConsoleReference{}
	type args struct {
		cap types.Capability
	}

	type want struct {
		tableName1 string
		tableName2 string
		errMsg     string
	}
	testcases := map[string]struct {
		args args
		want want
	}{
		"normal": {
			args: args{
				cap: types.Capability{
					TerraformConfiguration: `
resource "alicloud_oss_bucket" "bucket-acl" {
  bucket = var.bucket
  acl = var.acl
}

output "BUCKET_NAME" {
  value = "${alicloud_oss_bucket.bucket-acl.bucket}.${alicloud_oss_bucket.bucket-acl.extranet_endpoint}"
}

variable "bucket" {
  description = "OSS bucket name"
  default = "vela-website"
  type = string
}

variable "acl" {
  description = "OSS bucket ACL, supported 'private', 'public-read', 'public-read-write'"
  default = "private"
  type = string
}
`,
				},
			},
			want: want{
				errMsg:     "",
				tableName1: "### Properties",
				tableName2: "#### writeConnectionSecretToRef",
			},
		},
		"configuration is in git remote": {
			args: args{
				cap: types.Capability{
					TerraformConfiguration: "https://github.com/zzxwill/terraform-alibaba-eip.git",
					ConfigurationType:      "remote",
				},
			},
			want: want{
				errMsg:     "",
				tableName1: "### Properties",
				tableName2: "#### writeConnectionSecretToRef",
			},
		},
		"configuration is not valid": {
			args: args{
				cap: types.Capability{
					TerraformConfiguration: `abc`,
				},
			},
			want: want{
				errMsg: "failed to generate capability properties: :1,1-4: Argument or block definition required; An " +
					"argument or block definition is required here. To set an argument, use the equals sign \"=\" to " +
					"introduce the argument value.",
			},
		},
	}
	for name, tc := range testcases {
		consoleRef, err := ref.GenerateTerraformCapabilityProperties(tc.args.cap)
		var errMsg string
		if err != nil {
			errMsg = err.Error()
			if diff := cmp.Diff(tc.want.errMsg, errMsg, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGenerateTerraformCapabilityProperties(...): -want error, +got error:\n%s\n", name, diff)
			}
		} else {
			if diff := cmp.Diff(len(consoleRef), 2); diff != "" {
				t.Errorf("\n%s\nGenerateTerraformCapabilityProperties(...): -want, +got:\n%s\n", name, diff)
			}
			if diff := cmp.Diff(tc.want.tableName1, consoleRef[0].TableName); diff != "" {
				t.Errorf("\n%s\nGenerateTerraformCapabilityProperties(...): -want, +got:\n%s\n", name, diff)
			}
			if diff := cmp.Diff(tc.want.tableName2, consoleRef[1].TableName); diff != "" {
				t.Errorf("\n%s\nGexnerateTerraformCapabilityProperties(...): -want, +got:\n%s\n", name, diff)
			}
		}
	}
}

func TestPrepareTerraformOutputs(t *testing.T) {
	type args struct {
		tableName     string
		parameterList []ReferenceParameter
	}

	param := ReferenceParameter{}
	param.Name = "ID"
	param.Usage = "Identity of the cloud resource"

	testcases := []struct {
		args   args
		expect string
	}{
		{
			args: args{
				tableName:     "",
				parameterList: nil,
			},
			expect: "",
		},
		{
			args: args{
				tableName:     "abc",
				parameterList: []ReferenceParameter{param},
			},
			expect: "\n\nabc\n\nName | Description\n------------ | ------------- \n ID | Identity of the cloud resource\n",
		},
	}
	ref := &MarkdownReference{}
	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			content := ref.prepareTerraformOutputs(tc.args.tableName, tc.args.parameterList)
			if content != tc.expect {
				t.Errorf("prepareTerraformOutputs(...): -want, +got:\n%s\n", cmp.Diff(tc.expect, content))
			}
		})
	}
}

func TestMakeReadableTitle(t *testing.T) {
	type args struct {
		title string
	}
	testcases := []struct {
		args args
		want string
	}{
		{
			args: args{
				title: "abc",
			},
			want: "Abc",
		},
		{
			args: args{
				title: "abc-def",
			},
			want: "Abc-Def",
		},
		{
			args: args{
				title: "alibaba-def-ghi",
			},
			want: "Alibaba Cloud DEF-GHI",
		},
	}
	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			title := makeReadableTitle(tc.args.title)
			if title != tc.want {
				t.Errorf("makeReadableTitle(...): -want, +got:\n%s\n", cmp.Diff(tc.want, title))
			}
		})
	}
}
