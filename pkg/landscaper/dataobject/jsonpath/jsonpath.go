// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jsonpath

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/jsonpath"
)

func GetValue(text string, data interface{}, out interface{}) error {
	if !strings.HasPrefix(text, ".") {
		text = "." + text
	}
	jp := jsonpath.New("get")
	if err := jp.Parse(fmt.Sprintf("{%s}", text)); err != nil {
		return err
	}

	res := bytes.NewBuffer([]byte{})
	if err := jp.Execute(res, data); err != nil {
		return err
	}

	// do not try to marshal into nil
	if out == nil {
		return nil
	}

	return yaml.Unmarshal(res.Bytes(), out)
}

// Construct creates a map for the given jsonpath
// the value if the resulting map is set to the given value paramter
func Construct(text string, value interface{}) (map[string]interface{}, error) {
	if !strings.HasPrefix(text, ".") {
		text = "." + text
	}
	parser, err := jsonpath.Parse("construct", fmt.Sprintf("{%s}", text))
	if err != nil {
		return nil, err
	}

	out := make(map[string]interface{})
	if _, err := constructWalk(out, parser.Root, value); err != nil {
		return nil, err
	}
	return out, nil
}

func constructWalk(input map[string]interface{}, nodes *jsonpath.ListNode, value interface{}) (map[string]interface{}, error) {
	var (
		err     error
		fldPath = field.NewPath("")
	)
	curValue := input
	for i, node := range nodes.Nodes {
		switch n := node.(type) {
		case *jsonpath.ListNode:
			curValue, err = constructWalk(curValue, n, value)
			if err != nil {
				return curValue, err
			}
		case *jsonpath.FieldNode:
			newValue := make(map[string]interface{}, 0)
			fldPath = fldPath.Child(n.Value)
			curValue[n.Value] = newValue

			// if the node is the last in the list we can add the value
			if i == len(nodes.Nodes)-1 {
				curValue[n.Value] = value
				return curValue, nil
			}

			curValue = newValue
		default:
			return curValue, field.NotSupported(fldPath, node.Type(), []string{jsonpath.NodeTypeName[jsonpath.NodeList], jsonpath.NodeTypeName[jsonpath.NodeField]})
		}
	}

	return curValue, nil
}