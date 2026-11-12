/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	_ "embed"
	"fmt"
	"regexp"
	"sync"

	"gopkg.in/yaml.v3"
)

// taxonomyYAML is the API Platform feature taxonomy, embedded byte-for-byte
// from the product's taxonomy document. It is the source of truth for every
// feature code, metadata value and dimension this package sends: an occurrence
// is checked against it before it can leave the process, because the Moesif
// collector proxy forwards whatever it is given and validates nothing.
//
// Update it by replacing the file wholesale with the reviewed taxonomy; never
// edit a feature code or dimension here to make a call site compile.
//
//go:embed taxonomy/api-platform-taxonomy.yaml
var taxonomyYAML []byte

// DimensionType is the type a taxonomy dimension declares.
type DimensionType string

// Tracking platforms a taxonomy feature may list.
const (
	PlatformWeb = "web"
	PlatformAPI = "api"
)

const (
	DimensionEnum    DimensionType = "enum"
	DimensionBoolean DimensionType = "boolean"
	DimensionString  DimensionType = "string"
)

// reservedMetadataKeys are metadata keys a dimension may not use. This package
// fills the first four on every API event (environment when configured), as
// Agent Manager's API events do; the rest are keys WSO2 Cloud's feature
// reports fill. Reserving them keeps a
// dimension from shadowing or contradicting them, so one feature reads the
// same in Moesif whichever team reported it.
var reservedMetadataKeys = map[string]bool{
	"growth_action":      true,
	"platform":           true,
	"source":             true,
	"environment":        true,
	"action":             true,
	"category":           true,
	"tier":               true,
	"event_category":     true,
	"initiating_product": true,
}

var (
	featureCodePattern   = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9]+(-[a-z0-9]+)*)+$`)
	dimensionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	// An enum value is a lowercase code, hyphen- or underscore-separated; the
	// underscore admits protocol literals such as the OAuth grant type
	// "client_credentials".
	enumValuePattern = regexp.MustCompile(`^[a-z0-9]+([-_][a-z0-9]+)*$`)
)

// Feature is one taxonomy entry (a feature or a sub-feature), reduced to what
// a producer needs.
type Feature struct {
	Code          string
	Name          string
	Description   string
	Category      string
	Tier          string
	Status        string
	EventName     string
	EventCategory string
	// Platforms are where the taxonomy says the feature is tracked from:
	// "web" (a browser) and/or "api" (a backend handling an API call).
	Platforms  map[string]bool
	Dimensions map[string]Dimension
}

// Dimension is one declared dimension of a feature.
type Dimension struct {
	Name     string
	Type     DimensionType
	Values   map[string]bool // enum members; nil for non-enum types
	Required bool
}

// Taxonomy is a parsed, structurally validated feature taxonomy.
type Taxonomy struct {
	ProductID     string
	ProductName   string
	SchemaVersion string
	features      map[string]*Feature
}

// Feature returns the feature or sub-feature with the given code.
func (t *Taxonomy) Feature(code string) (*Feature, bool) {
	f, ok := t.features[code]
	return f, ok
}

// Len returns the number of features and sub-features in the taxonomy.
func (t *Taxonomy) Len() int { return len(t.features) }

type taxonomyDocument struct {
	Schema struct {
		Version string `yaml:"version"`
		ID      string `yaml:"id"`
	} `yaml:"schema"`
	Product struct {
		ID   string `yaml:"id"`
		Name string `yaml:"name"`
	} `yaml:"product"`
	Features []featureDocument `yaml:"features"`
}

type featureDocument struct {
	Code        string            `yaml:"code"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Category    string            `yaml:"category"`
	Tier        string            `yaml:"tier"`
	Status      string            `yaml:"status"`
	Tracking    trackingDocument  `yaml:"tracking"`
	SubFeatures []featureDocument `yaml:"sub_features"`
}

type trackingDocument struct {
	EventName     string              `yaml:"event_name"`
	EventCategory string              `yaml:"event_category"`
	Platforms     []string            `yaml:"platforms"`
	Dimensions    []dimensionDocument `yaml:"dimensions"`
}

type dimensionDocument struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Values   []string `yaml:"values"`
	Required bool     `yaml:"required"`
}

// ParseTaxonomy parses a feature taxonomy document and checks the structure
// this package relies on: unique, well-formed codes under the product's
// prefix, and dimensions whose names, types and enum values can be validated
// without guessing. It rejects the whole document on the first problem, since
// a taxonomy this package cannot read reliably must not be half-trusted.
func ParseTaxonomy(data []byte) (*Taxonomy, error) {
	var doc taxonomyDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse taxonomy: %w", err)
	}
	if doc.Product.ID == "" || doc.Product.Name == "" {
		return nil, fmt.Errorf("taxonomy has no product.id or product.name")
	}
	if len(doc.Features) == 0 {
		return nil, fmt.Errorf("taxonomy declares no features")
	}

	t := &Taxonomy{
		ProductID:     doc.Product.ID,
		ProductName:   doc.Product.Name,
		SchemaVersion: doc.Schema.Version,
		features:      map[string]*Feature{},
	}
	var add func(docs []featureDocument) error
	add = func(docs []featureDocument) error {
		for _, fd := range docs {
			f, err := t.buildFeature(fd)
			if err != nil {
				return err
			}
			if _, dup := t.features[f.Code]; dup {
				return fmt.Errorf("taxonomy declares feature %q more than once", f.Code)
			}
			t.features[f.Code] = f
			if err := add(fd.SubFeatures); err != nil {
				return err
			}
		}
		return nil
	}
	if err := add(doc.Features); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Taxonomy) buildFeature(fd featureDocument) (*Feature, error) {
	if !featureCodePattern.MatchString(fd.Code) {
		return nil, fmt.Errorf("taxonomy feature code %q is not a well-formed code", fd.Code)
	}
	if want := t.ProductID + "."; len(fd.Code) <= len(want) || fd.Code[:len(want)] != want {
		return nil, fmt.Errorf("taxonomy feature %q is not under product %q", fd.Code, t.ProductID)
	}
	if fd.Name == "" || fd.Category == "" || fd.Tier == "" || fd.Tracking.EventCategory == "" {
		return nil, fmt.Errorf("taxonomy feature %q is missing its name, category, tier or event_category", fd.Code)
	}

	f := &Feature{
		Code:          fd.Code,
		Name:          fd.Name,
		Description:   fd.Description,
		Category:      fd.Category,
		Tier:          fd.Tier,
		Status:        fd.Status,
		EventName:     fd.Tracking.EventName,
		EventCategory: fd.Tracking.EventCategory,
		Platforms:     map[string]bool{},
		Dimensions:    map[string]Dimension{},
	}
	for _, platform := range fd.Tracking.Platforms {
		if platform != PlatformWeb && platform != PlatformAPI {
			return nil, fmt.Errorf("taxonomy feature %q has unknown platform %q", fd.Code, platform)
		}
		f.Platforms[platform] = true
	}
	for _, dd := range fd.Tracking.Dimensions {
		d, err := buildDimension(fd.Code, dd)
		if err != nil {
			return nil, err
		}
		if _, dup := f.Dimensions[d.Name]; dup {
			return nil, fmt.Errorf("taxonomy feature %q declares dimension %q more than once", fd.Code, d.Name)
		}
		f.Dimensions[d.Name] = d
	}
	return f, nil
}

func buildDimension(code string, dd dimensionDocument) (Dimension, error) {
	if !dimensionNamePattern.MatchString(dd.Name) {
		return Dimension{}, fmt.Errorf("taxonomy feature %q has a malformed dimension name %q", code, dd.Name)
	}
	if reservedMetadataKeys[dd.Name] {
		return Dimension{}, fmt.Errorf("taxonomy feature %q uses reserved metadata key %q as a dimension", code, dd.Name)
	}
	d := Dimension{Name: dd.Name, Type: DimensionType(dd.Type), Required: dd.Required}
	switch d.Type {
	case DimensionEnum:
		if len(dd.Values) == 0 {
			return Dimension{}, fmt.Errorf("taxonomy feature %q enum dimension %q lists no values", code, dd.Name)
		}
		d.Values = make(map[string]bool, len(dd.Values))
		for _, v := range dd.Values {
			if !enumValuePattern.MatchString(v) {
				return Dimension{}, fmt.Errorf("taxonomy feature %q enum dimension %q has a malformed value", code, dd.Name)
			}
			if d.Values[v] {
				return Dimension{}, fmt.Errorf("taxonomy feature %q enum dimension %q lists a value twice", code, dd.Name)
			}
			d.Values[v] = true
		}
	case DimensionBoolean, DimensionString:
		if len(dd.Values) != 0 {
			return Dimension{}, fmt.Errorf("taxonomy feature %q dimension %q of type %s must not list values", code, dd.Name, dd.Type)
		}
	default:
		return Dimension{}, fmt.Errorf("taxonomy feature %q dimension %q has unsupported type %q", code, dd.Name, dd.Type)
	}
	return d, nil
}

var defaultTaxonomy = sync.OnceValues(func() (*Taxonomy, error) {
	return ParseTaxonomy(taxonomyYAML)
})

// DefaultTaxonomy returns the embedded API Platform taxonomy, parsed once.
func DefaultTaxonomy() (*Taxonomy, error) {
	return defaultTaxonomy()
}
