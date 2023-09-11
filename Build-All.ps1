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

function Get-LinkFlags($Component)
{
    $SentryDsn = @{
        agent       = $env:JUICE_AGENT_SENTRY_DSN;
        controller  = $env:JUICE_CONTROLLER_SENTRY_DSN;
        juicify     = $env:JUICE_JUICIFY_SENTRY_DSN;
    }

    $dsn = $SentryDsn[$Component]
    
    return @(
        "-X github.com/Juice-Labs/Juice-Labs/cmd/internal/build.Version=$Version";
        "-X github.com/Juice-Labs/Juice-Labs/pkg/sentry.SentryDsn=$dsn";
     ) | Join-String -Separator " "
}



if ($BuildDebug)
{
    & go build -o $Output/agent$Suffix -ldflags (Get-LinkFlags -Component "agent") -gcflags=all='-N -l' ./cmd/agent/main.go
    & go build -o $Output/controller$Suffix -ldflags (Get-LinkFlags -Component "controller") -gcflags=all='-N -l' ./cmd/controller/main.go
    & go build -o $Output/juicify$Suffix -ldflags (Get-LinkFlags -Component "juicify") -gcflags=all='-N -l' ./cmd/juicify/main.go
}
else
{
    & go build -o $Output/agent$Suffix -ldflags (Get-LinkFlags -Component "agent") ./cmd/agent/main.go
    & go build -o $Output/controller$Suffix -ldflags (Get-LinkFlags -Component "controller") ./cmd/controller/main.go
    & go build -o $Output/juicify$Suffix -ldflags (Get-LinkFlags -Component "juicify") ./cmd/juicify/main.go
}
