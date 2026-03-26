param(
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot

$pathsToRemove = @(
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    ".coverage",
    "htmlcov",
    "build",
    "dist"
)

function Remove-Target([string]$targetPath) {
    if (Test-Path $targetPath) {
        if ($DryRun) {
            Write-Host "[dry-run] would remove: $targetPath"
        } else {
            Remove-Item -Recurse -Force $targetPath
            Write-Host "removed: $targetPath"
        }
    }
}

foreach ($path in $pathsToRemove) {
    Remove-Target $path
}

$recursivePatterns = @("__pycache__", "*.egg-info")

foreach ($pattern in $recursivePatterns) {
    $matches = Get-ChildItem -Path . -Recurse -Force -Directory -Filter $pattern -ErrorAction SilentlyContinue
    foreach ($match in $matches) {
        Remove-Target $match.FullName
    }
}

Write-Host "cleanup done"
