/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package gpu

import (
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/Juice-Labs/pkg/api"
)

type Gpu struct {
	api.Gpu

	PciBus string `json:"pciBus"`

	availableVram uint64
	available     bool
}

type SelectedGpu struct {
	gpu *Gpu

	requiredVram uint64
}

type GpuSet []*Gpu
type SelectedGpuSet []SelectedGpu

func NewGpuSet(gpus []api.Gpu) GpuSet {
	gpuSet := GpuSet{}
	for _, gpu := range gpus {
		gpuSet = append(gpuSet, &Gpu{
			Gpu:           gpu,
			availableVram: gpu.Vram,
			available:     true,
		})
	}

	return gpuSet
}

func UnmarshalGpuSet(data []byte) (GpuSet, error) {
	var gpus GpuSet
	err := json.Unmarshal(data, &gpus)
	if err != nil {
		return gpus, err
	}

	for index := range gpus {
		gpus[index].available = true
	}

	return gpus, nil
}

func (gpus GpuSet) GetGpus() []api.Gpu {
	publicGpus := make([]api.Gpu, len(gpus))

	for i := 0; i < len(gpus); i++ {
		publicGpus[i] = gpus[i].Gpu
	}

	return publicGpus
}

func (gpus SelectedGpuSet) GetGpus() []api.Gpu {
	publicGpus := make([]api.Gpu, len(gpus))

	for i := 0; i < len(gpus); i++ {
		publicGpus[i] = gpus[i].gpu.Gpu
	}

	return publicGpus
}

func (gpus GpuSet) GetPciBusString() string {
	pciBus := ""

	if len(gpus) > 0 {
		pciBus = gpus[0].PciBus

		for i := 1; i < len(gpus); i++ {
			pciBus = fmt.Sprint(pciBus, ",", gpus[i].PciBus)
		}
	}

	return pciBus
}

func (gpus SelectedGpuSet) GetPciBusString() string {
	pciBus := ""

	if len(gpus) > 0 {
		pciBus = gpus[0].gpu.PciBus

		for i := 1; i < len(gpus); i++ {
			pciBus = fmt.Sprint(pciBus, ",", gpus[i].gpu.PciBus)
		}
	}

	return pciBus
}

func (gpus GpuSet) Find(requirements []api.GpuRequirements) (SelectedGpuSet, error) {
	if len(requirements) == 0 {
		return SelectedGpuSet{}, errors.New("must specify at least one GPU requirement")
	}

	availableGpuIndices := make([]int, 0, len(gpus))
	for index, gpu := range gpus {
		if gpu.available {
			availableGpuIndices = append(availableGpuIndices, index)
		}
	}

	var selectedGpus SelectedGpuSet
	for _, requirement := range requirements {
		for index, potentialGpuIndex := range availableGpuIndices {
			potentialGpu := gpus[potentialGpuIndex]

			if requirement.VendorId != 0 && potentialGpu.VendorId != requirement.VendorId {
				continue
			}

			if requirement.DeviceId != 0 && potentialGpu.DeviceId != requirement.DeviceId {
				continue
			}

			if requirement.VramRequired != 0 && potentialGpu.availableVram < requirement.VramRequired {
				continue
			}

			selectedGpus = append(selectedGpus, SelectedGpu{
				gpu:          potentialGpu,
				requiredVram: requirement.VramRequired,
			})

			availableGpuIndices = slices.Delete(availableGpuIndices, index, index+1)
		}
	}

	if len(selectedGpus) != len(requirements) {
		return SelectedGpuSet{}, errors.New("unable to find a matching set of GPUs")
	}

	for _, gpu := range selectedGpus {
		gpu.gpu.availableVram -= gpu.requiredVram
		gpu.gpu.available = false
	}

	return selectedGpus, nil
}

func (gpus GpuSet) Select(chosenGpus []api.Gpu) (SelectedGpuSet, error) {
	if len(chosenGpus) == 0 {
		return SelectedGpuSet{}, errors.New("must specify at least one chosen GPU")
	}

	availableGpus := map[int]*Gpu{}
	for index, gpu := range gpus {
		if gpu.available {
			availableGpus[index] = gpu
		}
	}

	var selectedGpus SelectedGpuSet
	for _, chosenGpu := range chosenGpus {
		availableGpu, available := availableGpus[chosenGpu.Index]
		if !available {
			return nil, fmt.Errorf("chosen GPU is not available")
		}

		if chosenGpu.Name != availableGpu.Name {
			return nil, fmt.Errorf("chosen GPU is does not have the correct Name")
		}

		if chosenGpu.VendorId != availableGpu.VendorId {
			return nil, fmt.Errorf("chosen GPU is does not have the correct VendorID")
		}

		if chosenGpu.DeviceId != availableGpu.DeviceId {
			return nil, fmt.Errorf("chosen GPU is does not have the correct DeviceID")
		}

		if chosenGpu.Vram != availableGpu.Vram {
			return nil, fmt.Errorf("chosen GPU is does not have the correct Vram")
		}

		selectedGpus = append(selectedGpus, SelectedGpu{
			gpu:          availableGpu,
			requiredVram: 0,
		})

		delete(availableGpus, chosenGpu.Index)
	}

	for _, gpu := range selectedGpus {
		gpu.gpu.availableVram -= gpu.requiredVram
		gpu.gpu.available = false
	}

	return selectedGpus, nil
}

func (gpus SelectedGpuSet) Release() {
	for _, gpu := range gpus {
		gpu.gpu.availableVram += gpu.requiredVram
		gpu.gpu.available = true
		gpu.requiredVram = 0
	}
}
