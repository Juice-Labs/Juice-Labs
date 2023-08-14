param (
    [Parameter()]
    [string]
    $Output = (Get-Location).Path,

    [Parameter()]
    [switch]
    $BuildDebug,

    [Parameter()]
    [string]
    $Version = "unset"
)

# Powershell 5 and below are available only on Windows
if ($PSVersionTable.PSVersion.Major -lt 7)
{
    $IsWindows = $true
    $IsLinux = $false
}

$Suffix = ""
if ($IsWindows)
{
    $Suffix = ".exe"
}

if ($BuildDebug)
{
    & go build -o $Output/agent$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" -gcflags=all='-N -l' ./cmd/agent/main.go
    & go build -o $Output/controller$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" -gcflags=all='-N -l' ./cmd/controller/main.go
    & go build -o $Output/juicify$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" -gcflags=all='-N -l' ./cmd/juicify/main.go
}
else
{
    & go build -o $Output/agent$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" ./cmd/agent/main.go
    & go build -o $Output/controller$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" ./cmd/controller/main.go
    & go build -o $Output/juicify$Suffix -ldflags "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version" ./cmd/juicify/main.go
}
