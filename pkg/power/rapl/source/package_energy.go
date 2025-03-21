/*
Copyright 2021.

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

package source

import (
	"fmt"
)

// RAPLEnergy defines set of energy per RAPL components in mJ
type RAPLEnergy struct {
	Core   uint64
	DRAM   uint64
	Uncore uint64
	Pkg    uint64
}

// RAPLComponentPower defines power per components in mJ
type RAPLPower struct {
	Core   uint64
	DRAM   uint64
	Uncore uint64
	Pkg    uint64
}

func (p RAPLPower) String() string {
	return fmt.Sprintf("Pkg: %d (Core: %d, Uncore: %d) DRAM: %d", p.Pkg, p.Core, p.Uncore, p.DRAM)
}
