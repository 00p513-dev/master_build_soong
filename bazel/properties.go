// Copyright 2020 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bazel

import (
	"fmt"
	"regexp"
	"sort"
)

// BazelTargetModuleProperties contain properties and metadata used for
// Blueprint to BUILD file conversion.
type BazelTargetModuleProperties struct {
	// The Bazel rule class for this target.
	Rule_class string `blueprint:"mutated"`

	// The target label for the bzl file containing the definition of the rule class.
	Bzl_load_location string `blueprint:"mutated"`
}

const BazelTargetModuleNamePrefix = "__bp2build__"

var productVariableSubstitutionPattern = regexp.MustCompile("%(d|s)")

// Label is used to represent a Bazel compatible Label. Also stores the original bp text to support
// string replacement.
type Label struct {
	Bp_text string
	Label   string
}

// LabelList is used to represent a list of Bazel labels.
type LabelList struct {
	Includes []Label
	Excludes []Label
}

// Append appends the fields of other labelList to the corresponding fields of ll.
func (ll *LabelList) Append(other LabelList) {
	if len(ll.Includes) > 0 || len(other.Includes) > 0 {
		ll.Includes = append(ll.Includes, other.Includes...)
	}
	if len(ll.Excludes) > 0 || len(other.Excludes) > 0 {
		ll.Excludes = append(other.Excludes, other.Excludes...)
	}
}

func UniqueBazelLabels(originalLabels []Label) []Label {
	uniqueLabelsSet := make(map[Label]bool)
	for _, l := range originalLabels {
		uniqueLabelsSet[l] = true
	}
	var uniqueLabels []Label
	for l, _ := range uniqueLabelsSet {
		uniqueLabels = append(uniqueLabels, l)
	}
	sort.SliceStable(uniqueLabels, func(i, j int) bool {
		return uniqueLabels[i].Label < uniqueLabels[j].Label
	})
	return uniqueLabels
}

func UniqueBazelLabelList(originalLabelList LabelList) LabelList {
	var uniqueLabelList LabelList
	uniqueLabelList.Includes = UniqueBazelLabels(originalLabelList.Includes)
	uniqueLabelList.Excludes = UniqueBazelLabels(originalLabelList.Excludes)
	return uniqueLabelList
}

const (
	// ArchType names in arch.go
	ARCH_ARM    = "arm"
	ARCH_ARM64  = "arm64"
	ARCH_X86    = "x86"
	ARCH_X86_64 = "x86_64"

	// OsType names in arch.go
	OS_ANDROID      = "android"
	OS_DARWIN       = "darwin"
	OS_FUCHSIA      = "fuchsia"
	OS_LINUX        = "linux_glibc"
	OS_LINUX_BIONIC = "linux_bionic"
	OS_WINDOWS      = "windows"
)

var (
	// This is the list of architectures with a Bazel config_setting and
	// constraint value equivalent. is actually android.ArchTypeList, but the
	// android package depends on the bazel package, so a cyclic dependency
	// prevents using that here.
	selectableArchs = []string{ARCH_X86, ARCH_X86_64, ARCH_ARM, ARCH_ARM64}

	// Likewise, this is the list of target operating systems.
	selectableTargetOs = []string{
		OS_ANDROID,
		OS_DARWIN,
		OS_FUCHSIA,
		OS_LINUX,
		OS_LINUX_BIONIC,
		OS_WINDOWS,
	}

	// A map of architectures to the Bazel label of the constraint_value
	// for the @platforms//cpu:cpu constraint_setting
	PlatformArchMap = map[string]string{
		ARCH_ARM:    "//build/bazel/platforms/arch:arm",
		ARCH_ARM64:  "//build/bazel/platforms/arch:arm64",
		ARCH_X86:    "//build/bazel/platforms/arch:x86",
		ARCH_X86_64: "//build/bazel/platforms/arch:x86_64",
	}

	// A map of target operating systems to the Bazel label of the
	// constraint_value for the @platforms//os:os constraint_setting
	PlatformOsMap = map[string]string{
		OS_ANDROID:      "//build/bazel/platforms/os:android",
		OS_DARWIN:       "//build/bazel/platforms/os:darwin",
		OS_FUCHSIA:      "//build/bazel/platforms/os:fuchsia",
		OS_LINUX:        "//build/bazel/platforms/os:linux",
		OS_LINUX_BIONIC: "//build/bazel/platforms/os:linux_bionic",
		OS_WINDOWS:      "//build/bazel/platforms/os:windows",
	}
)

// Arch-specific label_list typed Bazel attribute values. This should correspond
// to the types of architectures supported for compilation in arch.go.
type labelListArchValues struct {
	X86    LabelList
	X86_64 LabelList
	Arm    LabelList
	Arm64  LabelList
	Common LabelList
}

type labelListOsValues struct {
	Android     LabelList
	Darwin      LabelList
	Fuchsia     LabelList
	Linux       LabelList
	LinuxBionic LabelList
	Windows     LabelList
}

// LabelListAttribute is used to represent a list of Bazel labels as an
// attribute.
type LabelListAttribute struct {
	// The non-arch specific attribute label list Value. Required.
	Value LabelList

	// The arch-specific attribute label list values. Optional. If used, these
	// are generated in a select statement and appended to the non-arch specific
	// label list Value.
	ArchValues labelListArchValues

	// The os-specific attribute label list values. Optional. If used, these
	// are generated in a select statement and appended to the non-os specific
	// label list Value.
	OsValues labelListOsValues
}

// MakeLabelListAttribute initializes a LabelListAttribute with the non-arch specific value.
func MakeLabelListAttribute(value LabelList) LabelListAttribute {
	return LabelListAttribute{Value: UniqueBazelLabelList(value)}
}

// HasArchSpecificValues returns true if the attribute contains
// architecture-specific label_list values.
func (attrs *LabelListAttribute) HasConfigurableValues() bool {
	for _, arch := range selectableArchs {
		if len(attrs.GetValueForArch(arch).Includes) > 0 {
			return true
		}
	}

	for _, os := range selectableTargetOs {
		if len(attrs.GetValueForOS(os).Includes) > 0 {
			return true
		}
	}
	return false
}

func (attrs *LabelListAttribute) archValuePtrs() map[string]*LabelList {
	return map[string]*LabelList{
		ARCH_X86:    &attrs.ArchValues.X86,
		ARCH_X86_64: &attrs.ArchValues.X86_64,
		ARCH_ARM:    &attrs.ArchValues.Arm,
		ARCH_ARM64:  &attrs.ArchValues.Arm64,
	}
}

// GetValueForArch returns the label_list attribute value for an architecture.
func (attrs *LabelListAttribute) GetValueForArch(arch string) LabelList {
	var v *LabelList
	if v = attrs.archValuePtrs()[arch]; v == nil {
		panic(fmt.Errorf("Unknown arch: %s", arch))
	}
	return *v
}

// SetValueForArch sets the label_list attribute value for an architecture.
func (attrs *LabelListAttribute) SetValueForArch(arch string, value LabelList) {
	var v *LabelList
	if v = attrs.archValuePtrs()[arch]; v == nil {
		panic(fmt.Errorf("Unknown arch: %s", arch))
	}
	*v = value
}

func (attrs *LabelListAttribute) osValuePtrs() map[string]*LabelList {
	return map[string]*LabelList{
		OS_ANDROID:      &attrs.OsValues.Android,
		OS_DARWIN:       &attrs.OsValues.Darwin,
		OS_FUCHSIA:      &attrs.OsValues.Fuchsia,
		OS_LINUX:        &attrs.OsValues.Linux,
		OS_LINUX_BIONIC: &attrs.OsValues.LinuxBionic,
		OS_WINDOWS:      &attrs.OsValues.Windows,
	}
}

// GetValueForOS returns the label_list attribute value for an OS target.
func (attrs *LabelListAttribute) GetValueForOS(os string) LabelList {
	var v *LabelList
	if v = attrs.osValuePtrs()[os]; v == nil {
		panic(fmt.Errorf("Unknown os: %s", os))
	}
	return *v
}

// SetValueForArch sets the label_list attribute value for an OS target.
func (attrs *LabelListAttribute) SetValueForOS(os string, value LabelList) {
	var v *LabelList
	if v = attrs.osValuePtrs()[os]; v == nil {
		panic(fmt.Errorf("Unknown os: %s", os))
	}
	*v = value
}

// StringListAttribute corresponds to the string_list Bazel attribute type with
// support for additional metadata, like configurations.
type StringListAttribute struct {
	// The base value of the string list attribute.
	Value []string

	// Optional additive set of list values to the base value.
	ArchValues stringListArchValues
}

// Arch-specific string_list typed Bazel attribute values. This should correspond
// to the types of architectures supported for compilation in arch.go.
type stringListArchValues struct {
	X86    []string
	X86_64 []string
	Arm    []string
	Arm64  []string
	Common []string
}

// HasConfigurableValues returns true if the attribute contains
// architecture-specific string_list values.
func (attrs *StringListAttribute) HasConfigurableValues() bool {
	for _, arch := range selectableArchs {
		if len(attrs.GetValueForArch(arch)) > 0 {
			return true
		}
	}
	return false
}

func (attrs *StringListAttribute) archValuePtrs() map[string]*[]string {
	return map[string]*[]string{
		ARCH_X86:    &attrs.ArchValues.X86,
		ARCH_X86_64: &attrs.ArchValues.X86_64,
		ARCH_ARM:    &attrs.ArchValues.Arm,
		ARCH_ARM64:  &attrs.ArchValues.Arm64,
	}
}

// GetValueForArch returns the string_list attribute value for an architecture.
func (attrs *StringListAttribute) GetValueForArch(arch string) []string {
	var v *[]string
	if v = attrs.archValuePtrs()[arch]; v == nil {
		panic(fmt.Errorf("Unknown arch: %s", arch))
	}
	return *v
}

// SetValueForArch sets the string_list attribute value for an architecture.
func (attrs *StringListAttribute) SetValueForArch(arch string, value []string) {
	var v *[]string
	if v = attrs.archValuePtrs()[arch]; v == nil {
		panic(fmt.Errorf("Unknown arch: %s", arch))
	}
	*v = value
}

// TryVariableSubstitution, replace string substitution formatting within each string in slice with
// Starlark string.format compatible tag for productVariable.
func TryVariableSubstitutions(slice []string, productVariable string) ([]string, bool) {
	ret := make([]string, 0, len(slice))
	changesMade := false
	for _, s := range slice {
		newS, changed := TryVariableSubstitution(s, productVariable)
		ret = append(ret, newS)
		changesMade = changesMade || changed
	}
	return ret, changesMade
}

// TryVariableSubstitution, replace string substitution formatting within s with Starlark
// string.format compatible tag for productVariable.
func TryVariableSubstitution(s string, productVariable string) (string, bool) {
	sub := productVariableSubstitutionPattern.ReplaceAllString(s, "{"+productVariable+"}")
	return sub, s != sub
}
