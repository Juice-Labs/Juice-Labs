/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package playnite

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/app"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

func NewGpuMetricsConsumer(agent *app.Agent) (gpu.MetricsConsumerFn, error) {
	port, err := agent.Server.Port()
	if err != nil {
		return nil, err
	}

	netInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	broadcastAddresses := make([]net.Addr, 0)
	for _, netInterface := range netInterfaces {
		if netInterface.Flags&net.FlagRunning != 0 && netInterface.Flags&net.FlagBroadcast != 0 {
			addrs, err := netInterface.Addrs()
			if err != nil {
				return nil, err
			} else {
				for _, addrAny := range addrs {
					addr, err := utilities.Cast[*net.IPNet](addrAny)
					if err != nil {
						logger.Warning(err)
					} else if ipv4 := addr.IP.To4(); ipv4 != nil {
						broadcastIp := ipv4.Mask(addr.Mask)
						binary.BigEndian.PutUint32(broadcastIp, binary.BigEndian.Uint32(broadcastIp)|^binary.BigEndian.Uint32(net.IP(addr.Mask).To4()))

						broadcastAddresses = append(broadcastAddresses, &net.UDPAddr{
							IP:   broadcastIp,
							Port: 43210,
						})
					}
				}
			}
		}
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}

	udpConn, err := utilities.Cast[*net.UDPConn](conn)
	if err != nil {
		return nil, err
	}

	nonce := 0
	return func(metrics []gpu.Metrics) {
		bytes, err := json.Marshal(metrics)
		if err == nil {
			data := fmt.Sprintf(`{"action":"UPDATE","uuid":"%s","port":%d,"nonce":%d,"hostname":"%s","data":%s}`, agent.Id, port, nonce, agent.Hostname, string(bytes))

			for _, addr := range broadcastAddresses {
				udpConn.WriteTo([]byte(data), addr)
			}

			nonce++
		} else {
			logger.Warning(err)
		}
	}, nil
}
