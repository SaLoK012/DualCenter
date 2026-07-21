param(
    [switch]$SkipSigning
)

$ErrorActionPreference = "Stop"

$metadata = Get-Content ".\internal\version\version.json" -Raw | ConvertFrom-Json
$versionLabel = "v$($metadata.version)-$($metadata.label)"
$setupVersioned = "dist\DualCenter-Setup-$versionLabel.exe"
$checksum = "dist\DualCenter-$versionLabel-SHA256SUMS.txt"
$payload = "installer\payload\DualCenter.exe"
$generatedResources = @("resource_amd64.syso", "installer\resource_amd64.syso")

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

function Invoke-Go {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & go @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "go $($Arguments -join ' ') falhou com código $LASTEXITCODE"
    }
}

function Sign-IfConfigured {
    param([string]$Path)
    if ($SkipSigning) {
        return
    }
    $thumbprint = $env:DUALCENTER_SIGN_CERT_SHA1
    $timestamp = $env:DUALCENTER_TIMESTAMP_URL
    if ([string]::IsNullOrWhiteSpace($thumbprint) -or [string]::IsNullOrWhiteSpace($timestamp)) {
        Write-Warning "Assinatura não configurada. Defina DUALCENTER_SIGN_CERT_SHA1 e DUALCENTER_TIMESTAMP_URL ou use -SkipSigning."
        return
    }
    $signTool = Get-Command "signtool.exe" -ErrorAction Stop
    & $signTool.Source sign /sha1 $thumbprint /fd SHA256 /tr $timestamp /td SHA256 $Path
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao assinar $Path"
    }
    & $signTool.Source verify /pa /v $Path
    if ($LASTEXITCODE -ne 0) {
        throw "A assinatura de $Path não pôde ser verificada"
    }
}

Remove-Item -Recurse -Force "dist" -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
New-Item -ItemType Directory -Force -Path "installer\payload" | Out-Null
Remove-Item -Force $payload -ErrorAction SilentlyContinue

try {
    Invoke-Go run .\cmd\resourcegen --icon .\assets\DualCenter.ico --manifest .\app.manifest --out .\resource_amd64.syso --description DualCenter --filename DualCenter.exe
    Invoke-Go run .\cmd\resourcegen --icon .\assets\DualCenter.ico --manifest .\installer\setup.manifest --out .\installer\resource_amd64.syso --description "DualCenter Setup" --filename "DualCenter-Setup-$versionLabel.exe"

    $goFiles = Get-ChildItem -Path "." -Filter "*.go" -File -Recurse | ForEach-Object { $_.FullName }
    $unformatted = & gofmt -l @goFiles
    if ($LASTEXITCODE -ne 0) {
        throw "gofmt falhou"
    }
    if ($unformatted) {
        throw "Arquivos Go fora do padrão:`n$($unformatted -join "`n")"
    }

    Invoke-Go vet -buildvcs=false . ./internal/... ./cmd/...
    Invoke-Go test -buildvcs=false . ./internal/... ./cmd/...
    Invoke-Go build -buildvcs=false -trimpath -ldflags "-H=windowsgui -s -w -buildid=" -o $payload .
    Sign-IfConfigured $payload

    Invoke-Go vet -buildvcs=false ./installer
    Invoke-Go test -buildvcs=false ./installer

    Invoke-Go build -buildvcs=false -trimpath -ldflags "-H=windowsgui -s -w -buildid=" -o $setupVersioned .\installer
    Sign-IfConfigured $setupVersioned

    $hash = (Get-FileHash $setupVersioned -Algorithm SHA256).Hash.ToLower()
    "$hash  $(Split-Path $setupVersioned -Leaf)" | Set-Content -Encoding ascii $checksum
}
finally {
    Remove-Item -Force $payload -ErrorAction SilentlyContinue
    Remove-Item -Force -Path $generatedResources -ErrorAction SilentlyContinue
}

Write-Host "Build concluído."
Write-Host "- $setupVersioned"
Write-Host "- $checksum"
