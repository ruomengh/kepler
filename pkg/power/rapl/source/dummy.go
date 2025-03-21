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

type PowerDummy struct{}

func (r *PowerDummy) IsSupported() bool {
	return true
}

func (r *PowerDummy) StopPower() {
}

func (r *PowerDummy) GetEnergyFromDram() (uint64, error) {
	return 0, nil
}

func (r *PowerDummy) GetEnergyFromCore() (uint64, error) {
	return 0, nil
}

func (r *PowerDummy) GetEnergyFromUncore() (uint64, error) {
	return 0, nil
}

func (r *PowerDummy) GetEnergyFromPackage() (uint64, error) {
	return 0, nil
}

func (r *PowerDummy) GetRAPLEnergy() map[int]RAPLEnergy {
	return map[int]RAPLEnergy{}
}
