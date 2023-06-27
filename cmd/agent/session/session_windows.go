/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"reflect"
	"syscall"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/session/windows"
)

func setupIpc() (*os.File, *os.File, error) {
	return os.Pipe()
}

func inheritFile(cmd *exec.Cmd, f *os.File) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles, syscall.Handle(f.Fd()))
}

func (session *Session) forwardSocket(rawConn syscall.RawConn) error {
	conn := reflect.Indirect(reflect.ValueOf(rawConn))
	netFD := reflect.Indirect(conn.FieldByName("fd"))
	pfd := netFD.FieldByName("pfd")
	fd := syscall.Handle(pfd.FieldByName("Sysfd").Uint())

	var protocolInfo syscall.WSAProtocolInfo
	err := windows.WSADuplicateSocketW(fd, uint32(session.cmd.Process.Pid), &protocolInfo)
	if err != nil {
		return err
	}

	var protocolInfoBytes []byte
	buffer := bytes.NewBuffer(protocolInfoBytes)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ServiceFlags1)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ServiceFlags2)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ServiceFlags3)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ServiceFlags4)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderFlags)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderId.Data1)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderId.Data2)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderId.Data3)
	for i := 0; i < len(protocolInfo.ProviderId.Data4); i++ {
		binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderId.Data4[i])
	}
	binary.Write(buffer, binary.LittleEndian, protocolInfo.CatalogEntryId)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProtocolChain.ChainLen)
	for i := 0; i < len(protocolInfo.ProtocolChain.ChainEntries); i++ {
		binary.Write(buffer, binary.LittleEndian, protocolInfo.ProtocolChain.ChainEntries[i])
	}
	binary.Write(buffer, binary.LittleEndian, protocolInfo.Version)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.AddressFamily)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.MaxSockAddr)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.MinSockAddr)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.SocketType)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.Protocol)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProtocolMaxOffset)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.NetworkByteOrder)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.SecurityScheme)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.MessageSize)
	binary.Write(buffer, binary.LittleEndian, protocolInfo.ProviderReserved)
	for i := 0; i < len(protocolInfo.ProtocolName); i++ {
		binary.Write(buffer, binary.LittleEndian, protocolInfo.ProtocolName[i])
	}

	_, err = session.cmdPipe.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	return nil
}
