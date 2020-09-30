/*
Copyright (C)  2020 Yahoo Japan Corporation Athenz team.

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

package authorizerd

import (
	"errors"
	"net/url"
	"strings"
)

const (
	placeholderPrefix = "{"
	placeholderSuffix = "}"
)

type Translator interface {
	Translate(domain, method, path, query string) (string, string, error)
	Validate() error
}

type Validated struct {
	Value       string
	Placeholder string
}

type Rule struct {
	Method        string `yaml:"method"`
	Path          string `yaml:"path"`
	Action        string `yaml:"action"`
	Resource      string `yaml:"resource"`
	splitPaths    []Validated
	queryValueMap map[string]Validated
}

type MappingRules struct {
	Rules map[string][]Rule
}

func (mr *MappingRules) Validate() error {
	for domain, rules := range mr.Rules {
		if domain == "" {
			return errors.New("k is empty or v is nil")
		}
		if rules == nil {
			return errors.New("")
		}

		for i, rule := range rules {
			if rule.Method == "" || rule.Path == "" || rule.Action == "" || rule.Resource == "" {
				return errors.New("Rule is empty")
			}
			if !strings.HasPrefix(rule.Path, "/") {
				return errors.New("path is not started slash")
			}
			if rule.Path == "/" {
				return errors.New("")
			}

			pathQuery := strings.SplitN(rule.Path, "?", 2)

			splitPaths := strings.Split(pathQuery[0], "/")
			rules[i].splitPaths = make([]Validated, len(splitPaths))
			for j, path := range splitPaths {
				if ok, err := rules[i].isPlaceholder(path); ok {
					rules[i].splitPaths[j] = Validated{Placeholder: path}
				} else {
					if err != nil {
						return errors.New("")
					}
					rules[i].splitPaths[j] = Validated{Value: path}
				}
			}

			rules[i].queryValueMap = make(map[string]Validated)
			if len(pathQuery) == 1 {
				continue
			}

			values, err := url.ParseQuery(pathQuery[1])
			if err != nil {
				return err
			}

			for param, val := range values {
				if len(val) != 1 {
					return errors.New("len(val) != 1")
				}
				if ok, err := rules[i].isPlaceholder(val[0]); ok {
					rules[i].queryValueMap[param] = Validated{Placeholder: val[0]}
				} else {
					if err != nil {
						return errors.New("")
					}
					rules[i].queryValueMap[param] = Validated{Value: val[0]}
				}
			}
		}
	}
	return nil
}

func (mr *MappingRules) Translate(domain, method, path, query string) (string, string, error) {
	if mr.Rules == nil {
		return method, path, nil
	}

OUTER:
	for _, rule := range mr.Rules[domain] {
		if rule.Method == method {
			requestedPaths := strings.Split(path, "/")
			if len(requestedPaths) != len(rule.splitPaths) {
				continue
			}

			requestedQuery, err := url.ParseQuery(query)
			if err != nil {
				return method, path, err
			}
			if len(requestedQuery) != len(rule.queryValueMap) {
				continue
			}

			placeholderMap := make(map[string]string)
			for i, reqPath := range requestedPaths {
				if rule.splitPaths[i].Placeholder != "" {
					placeholderMap[rule.splitPaths[i].Placeholder] = reqPath
				} else if reqPath != rule.splitPaths[i].Value {
					continue OUTER
				}
			}

			for reqQuery, reqVal := range requestedQuery {
				if len(reqVal) != 1 {
					continue OUTER
				}
				if v, ok := rule.queryValueMap[reqQuery]; ok {
					if v.Placeholder != "" {
						placeholderMap[v.Placeholder] = reqVal[0]
					} else if reqVal[0] != v.Value {
						continue OUTER
					}
				} else {
					continue OUTER
				}
			}

			replacedRes := rule.Resource
			for placeholder, v := range placeholderMap {
				replacedRes = strings.ReplaceAll(replacedRes, placeholder, v)
			}
			return rule.Action, replacedRes, nil
		}
	}
	return method, path, nil
}

func (r *Rule) isPlaceholder(s string) (bool, error) {
	if s == placeholderPrefix+placeholderSuffix {
		return false, errors.New("placeholder is empty")
	}

	if strings.HasPrefix(s, placeholderPrefix) && strings.HasSuffix(s, placeholderSuffix) {
		if r.splitPaths != nil {
			for _, v := range r.splitPaths {
				if v.Placeholder == s {
					return false, errors.New("duplicated")
				}
			}
		}
		if r.queryValueMap != nil {
			for _, v := range r.queryValueMap {
				if v.Placeholder == s {
					return false, errors.New("duplicated")
				}
			}
		}
		return true, nil
	} else {
		return false, nil
	}
}
