$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$webuiDir = Join-Path $projectDir "webui"

function Stop-ProcessTree {
    param([System.Diagnostics.Process]$Process)
    if ($null -eq $Process -or $Process.HasExited) { return }
    # Stop only the exact process tree started by this script. go run and npm
    # both create child processes which must be stopped together.
    & taskkill.exe /PID $Process.Id /T /F 2>$null | Out-Null
}

function Start-DevelopmentServices {
    Write-Host ""
    Write-Host "Starting DomainSprite in this console..." -ForegroundColor Cyan
    $frontend = Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev") -WorkingDirectory $webuiDir -NoNewWindow -PassThru
    try {
        $backend = Start-Process -FilePath "go.exe" -ArgumentList @("run", ".") -WorkingDirectory $projectDir -NoNewWindow -PassThru
    }
    catch {
        Stop-ProcessTree $frontend
        throw
    }

    try {
        while (-not $frontend.HasExited -and -not $backend.HasExited) {
            Start-Sleep -Milliseconds 300
        }
        if ($backend.HasExited) {
			$backend.WaitForExit()
			$backend.Refresh()
            Write-Host "Backend exited with code $($backend.ExitCode)." -ForegroundColor Yellow
        }
        if ($frontend.HasExited) {
			$frontend.WaitForExit()
			$frontend.Refresh()
            Write-Host "Frontend exited with code $($frontend.ExitCode)." -ForegroundColor Yellow
        }
    }
    finally {
        Stop-ProcessTree $backend
        Stop-ProcessTree $frontend
    }
}

try {
    do {
        Start-DevelopmentServices
        $answer = Read-Host "Press R to restart both services, or Enter to exit"
    } while ($answer.Trim().Equals("r", [System.StringComparison]::OrdinalIgnoreCase))
}
finally {
    Write-Host "DomainSprite development services stopped."
}
