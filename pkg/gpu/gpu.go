/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package gpu

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

type Gpu struct {
	restapi.Gpu

	vramAvailable uint64
}

type GpuSet struct {
	gpus []*Gpu
}

type SelectedGpu struct {
	gpu *Gpu

	vramRequired uint64
}

type SelectedGpuSet struct {
	gpus []SelectedGpu

	released bool
}

func NewGpuSet(apiGpus []restapi.Gpu) *GpuSet {
	gpus := make([]*Gpu, 0)
	for _, gpu := range apiGpus {
		gpus = append(gpus, &Gpu{
			Gpu:           gpu,
			vramAvailable: gpu.Vram,
		})
	}

	return &GpuSet{
		gpus: gpus,
	}
}

func NewGpuSetFromJson(data []byte) (*GpuSet, error) {
	var apiGpus []restapi.Gpu
	err := json.Unmarshal(data, &apiGpus)
	if err != nil {
		return nil, fmt.Errorf("NewGpuSetFromJson: invalid json, %s", string(data))
	}

	if len(apiGpus) == 0 {
		return nil, errors.New("NewGpuSetFromJson: json does not specify any GPUs")
	}

	gpus := make([]*Gpu, 0)
	for _, apiGpu := range apiGpus {
		gpus = append(gpus, &Gpu{
			Gpu:           apiGpu,
			vramAvailable: apiGpu.Vram,
		})
	}

	return &GpuSet{
		gpus: gpus,
	}, nil
}

func (gpuSet *GpuSet) Count() int {
	return len(gpuSet.gpus)
}

func (gpuSet *SelectedGpuSet) Count() int {
	return len(gpuSet.gpus)
}

func (gpuSet *GpuSet) GetGpus() []restapi.Gpu {
	publicGpus := make([]restapi.Gpu, len(gpuSet.gpus))
	for index, gpu := range gpuSet.gpus {
		publicGpus[index] = gpu.Gpu
	}

	return publicGpus
}

func (gpuSet *SelectedGpuSet) GetGpus() []restapi.SessionGpu {
	publicGpus := make([]restapi.SessionGpu, len(gpuSet.gpus))
	for index, gpu := range gpuSet.gpus {
		publicGpus[index] = restapi.SessionGpu{
			Index:        gpu.gpu.Index,
			VramRequired: gpu.vramRequired,
		}
	}

	return publicGpus
}

func (gpuSet *GpuSet) GetPciBusString() string {
	pciBus := ""

	if len(gpuSet.gpus) > 0 {
		pciBus = gpuSet.gpus[0].PciBus

		for i := 1; i < len(gpuSet.gpus); i++ {
			pciBus = fmt.Sprint(pciBus, ",", gpuSet.gpus[i].PciBus)
		}
	}

	return pciBus
}

func (gpuSet *SelectedGpuSet) GetPciBusString() string {
	pciBus := ""

	if len(gpuSet.gpus) > 0 {
		pciBus = gpuSet.gpus[0].gpu.PciBus

		for i := 1; i < len(gpuSet.gpus); i++ {
			pciBus = fmt.Sprint(pciBus, ",", gpuSet.gpus[i].gpu.PciBus)
		}
	}

	return pciBus
}

func (gpuSet *GpuSet) Find(requirements []restapi.GpuRequirements) (*SelectedGpuSet, error) {
	if len(requirements) == 0 {
		logger.Panic("GpuSet.Find: expected at least one GPU requirement")
	}

	// Currently, this algorithm will choose the first GPU that matches both VRAM and the PCIBus, if specified.
	// This algorithm does not allow GPUs to be reused though there is no reason why the GPUs could not be reused,
	// though not preferrable if other GPUs are available.

	// TODO: Better matching algorithm. Reuse of the same GPU can be done but should be the last option

	availableGpus := map[int]*Gpu{}
	for index, gpu := range gpuSet.gpus {
		availableGpus[index] = gpu
	}

	selectedGpus := make([]SelectedGpu, 0)
	for _, requirement := range requirements {
		for index, potentialGpu := range availableGpus {
			if requirement.VramRequired != 0 && potentialGpu.vramAvailable < requirement.VramRequired {
				continue
			}

			if requirement.PciBus != "" {
				potential := NewPCIAddressFromString(potentialGpu.PciBus)
				required := NewPCIAddressFromString(requirement.PciBus)
				if potential != required {
					continue
				}
			}

			selectedGpus = append(selectedGpus, SelectedGpu{
				gpu:          potentialGpu,
				vramRequired: requirement.VramRequired,
			})

			delete(availableGpus, index)
		}
	}

	if len(selectedGpus) < len(requirements) {
		return nil, errors.New("unable to find a matching set of GPUs")
	}

	for _, gpu := range selectedGpus {
		gpu.gpu.vramAvailable -= gpu.vramRequired
	}

	return &SelectedGpuSet{
		gpus:     selectedGpus,
		released: false,
	}, nil
}

func (gpuSet *GpuSet) Select(chosenGpus []restapi.SessionGpu) (*SelectedGpuSet, error) {
	if len(chosenGpus) == 0 {
		logger.Panic("GpuSet.Select: expected at least one chosen GPU")
	}

	selectedGpus := make([]SelectedGpu, 0)
	for _, chosenGpu := range chosenGpus {
		gpu := gpuSet.gpus[chosenGpu.Index]
		selectedGpus = append(selectedGpus, SelectedGpu{
			gpu:          gpu,
			vramRequired: chosenGpu.VramRequired,
		})
		gpu.vramAvailable -= chosenGpu.VramRequired
	}

	return &SelectedGpuSet{
		gpus:     selectedGpus,
		released: false,
	}, nil
}

func (gpuSet *SelectedGpuSet) Release() {
	if gpuSet.released {
		logger.Panic("SelectedGpuSet.Release: release called twice")
	}

	for _, gpu := range gpuSet.gpus {
		gpu.gpu.vramAvailable += gpu.vramRequired
	}

	gpuSet.released = true
}
