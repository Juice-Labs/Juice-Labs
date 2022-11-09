**[Join our Discord Server](https://discord.gg/xWHXNX8b3V) for ideas, questions, and feedback**

## See [Welcome to Juice GPU-over-IP](https://github.com/Juice-Labs/Juice-Labs/wiki)

We're offering up our Community Version here for your use, which is governed by these [Terms and Conditions](https://github.com/Juice-Labs/juice-hub/wiki/Terms-and-Conditions).

## What is Juice?

Juice is **GPU-over-IP**: a software application that routes GPU workloads over standard networking, creating a **client-server model** where **virtual remote GPU capacity** is provided _from_ Server machines that have physical GPUs (GPU Hosts) _to_ Client machines that are running GPU-hungry applications (Application Hosts). A single GPU Host can service an arbitrary number of Application Hosts.

Client applications are unaware that the physical GPU is remote, and physical GPUs are unaware that the workloads they are servicing are remote -- therefore **no modifications are necessary to applications or hardware.**

## Why Juice?

GPU capacity is increasingly critical to major trends in computing, but its use is hampered by a major limitation: a GPU-hungry application can only run in the same physical machine as the GPU itself.  This limitation causes extreme local-resourcing problems -- there's either not enough (or none, depending on the size and power needs of the device), or GPU capacity sits idle and wasted (utilization is broadly estimated at below 15%).

**By abstracting application hosts from physical GPUs, Juice decouples GPU-consuming clients from GPU-providing servers:**

1. **Any client workload can access GPU from anywhere, creating new capabilities**
1. **GPU capacity is pooled and shared across wide areas -- GPU hardware scales independently of other computing resources**
1. **GPU utilization is driven much higher, and stranded capacity is rescued, by dynamically adding multiple clients to the same GPU based on resource needs and availability -- i.e. more workloads are served with the same GPU hardware**

***

Please go to [Welcome to Juice GPU-over-IP](https://github.com/Juice-Labs/Juice-Labs/wiki) for the full picture.
