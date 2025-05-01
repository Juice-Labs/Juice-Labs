
![Juice Logo](https://p198.p4.n0.cdn.getcloudapp.com/items/Jru8KW5r/e0c3cb8e-9175-4a44-a8e4-e373a775b995.png?v=9bd38bca5ef40f4e132c1fc0338878ce)

# Welcome to Juice!

See [our home page](https://www.juicelabs.co/) and [our docs site](https://juice-labs.github.io/juice-docs/docs/juice/intro) for the latest info.

## There is now a [free version](https://app.juicelabs.co/login?mode=signup) of the Juice product available.

# What is Juice?

Juice is **GPU-over-IP**: client/server software that allows GPUs to be used over a standard TCP/IP network.  Run the Juice server on a machine with a physical GPU, and then immediately access that GPU from any machine with the Juice client software.

Client applications are unaware that the physical GPU is remote and **no modifications are necessary to application software**.  Juice client and server software runs equally well on **physical machines**, **VMs**, and **containers** on both **Linux** and **Windows**.  The only hard requirements are a GPU to serve and a network connection between the client and server.

# Why Juice?

GPU capacity is increasingly critical to major trends in computing, but its use is hampered by a major limitation: a GPU-hungry application can only run in the same physical machine as the GPU itself.  This limitation causes extreme local-resourcing problems -- there's either not enough (or none, depending on the size and power needs of the device), or GPU capacity sits idle and wasted (utilization is broadly estimated at below 15%).

**By abstracting application hosts from physical GPUs, Juice decouples GPU-consuming clients from GPU-providing servers:**

1. **Any client workload can access GPU from anywhere, creating new capabilities**
1. **GPU capacity is pooled and shared across wide areas -- GPU hardware scales independently of other computing resources**
1. **GPU utilization is driven much higher, and stranded capacity is rescued, by dynamically adding multiple clients to the same GPU based on resource needs and availability -- i.e. more workloads are served with the same GPU hardware**
