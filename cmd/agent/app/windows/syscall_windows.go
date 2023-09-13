/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package windows

//go:generate go run ./mksyscall_windows.go -systemdll -output zsyscall_windows.go syscall_windows.go

const socket_error = uintptr(^uint32(0))

//sys	WSADuplicateSocketW(s Handle, processId uint32, protocolBuffer *WSAProtocolInfo) (err error) [failretval==socket_error] = ws2_32.WSADuplicateSocketW
