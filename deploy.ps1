<#
  TechET Pay - one-shot deploy for Windows PowerShell 5.1+ or PowerShell 7
    1. pushes this folder to a new GitHub repository
    2. creates a FREE web service on Render (Docker) that runs the Go server
    3. builds the Android APK on GitHub Actions and downloads it next to this script

  Run it from the folder that contains this file:
      powershell -ExecutionPolicy Bypass -File .\deploy.ps1

  Nothing secret is written to disk. API keys are only sent to Render / GitHub.
#>
[CmdletBinding()]
param(
    [string]$RepoName    = "techet-pay",
    [string]$ServiceName = "techet-pay",
    [string]$Region      = "frankfurt",
    [switch]$Private,
    [switch]$SkipRender,
    [switch]$SkipApk,
    [string]$ServerUrl   = ""     # use an existing server instead of creating one on Render
)

$ErrorActionPreference = "Continue"
try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch {}
Set-Location -Path $PSScriptRoot

function Step($m) { Write-Host ""; Write-Host "==> $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host "    OK   $m" -ForegroundColor Green }
function Note($m) { Write-Host "         $m" -ForegroundColor Gray }
function Warn($m) { Write-Host "    WARN $m" -ForegroundColor Yellow }
function Die($m)  { Write-Host ""; Write-Host "ERROR: $m" -ForegroundColor Red; exit 1 }

function Refresh-Path {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
}

function Need($cmd, $wingetId, $url) {
    if (Get-Command $cmd -ErrorAction SilentlyContinue) { return }
    Warn "$cmd is not installed."
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        $a = Read-Host "    Install $cmd now with winget? (Y/n)"
        if ($a -eq "" -or $a -match "^[Yy]") {
            winget install --id $wingetId -e --accept-source-agreements --accept-package-agreements
            Refresh-Path
        }
    }
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Die "$cmd is required. Install it from $url, open a NEW PowerShell window and run this script again."
    }
}

function Run-Native {
    param([string]$Exe, [string[]]$Arguments)
    & $Exe @Arguments
    if ($LASTEXITCODE -ne 0) { Die "'$Exe $($Arguments -join ' ')' failed (exit code $LASTEXITCODE)." }
}

function Plain($secure) {
    $b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringAuto($b) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) }
}

function Get-ErrBody($e) {
    try {
        if ($e.ErrorDetails -and $e.ErrorDetails.Message) { return $e.ErrorDetails.Message }
        $r = $e.Exception.Response
        if ($r -and ($r | Get-Member -Name GetResponseStream -ErrorAction SilentlyContinue)) {
            $sr = New-Object IO.StreamReader($r.GetResponseStream())
            return $sr.ReadToEnd()
        }
    } catch {}
    return $e.Exception.Message
}

Write-Host ""
Write-Host "  TechET Pay deploy" -ForegroundColor White
Write-Host "  GitHub  ->  Render (free)  ->  Android APK" -ForegroundColor Gray

# ------------------------------------------------------------------ 1. tools
Step "Checking tools"
Need "git" "Git.Git" "https://git-scm.com/download/win"
Need "gh"  "GitHub.cli" "https://cli.github.com"
Ok "git and GitHub CLI found"

# ------------------------------------------------------------------ 2. GitHub login
Step "Signing in to GitHub"
gh auth status *> $null
if ($LASTEXITCODE -ne 0) {
    Note "A browser window will open. Sign in and approve the access."
    Run-Native "gh" @("auth", "login", "--web", "--git-protocol", "https", "--scopes", "repo,workflow")
}
Run-Native "gh" @("auth", "setup-git")
$owner = (gh api user --jq ".login" | Out-String).Trim()
if (-not $owner) { Die "Could not read your GitHub username." }
Ok "Signed in as $owner"

# ------------------------------------------------------------------ 3. commit
Step "Preparing the repository"
if (-not (Test-Path ".git")) { Run-Native "git" @("init", "-b", "main") }
$gitName = (git config user.name | Out-String).Trim()
if (-not $gitName) {
    $id = (gh api user --jq ".id" | Out-String).Trim()
    git config user.name  $owner
    git config user.email "$id+$owner@users.noreply.github.com"
}
Run-Native "git" @("add", "-A")
$dirty = (git status --porcelain | Out-String).Trim()
if ($dirty) { Run-Native "git" @("commit", "-m", "TechET Pay: Go server, Render blueprint, Android app") }
Run-Native "git" @("branch", "-M", "main")
Ok "Files committed"

# ------------------------------------------------------------------ 4. GitHub repo
Step "Creating the GitHub repository $owner/$RepoName"
$full = "$owner/$RepoName"
gh repo view $full *> $null
if ($LASTEXITCODE -ne 0) {
    $vis = "--public"
    if ($Private) { $vis = "--private" }
    Run-Native "gh" @("repo", "create", $full, $vis, "--description", "TechET Pay - payment gateway sandbox (Go) with Android app")
    Ok "Repository created"
} else {
    Note "Repository already exists, reusing it."
}
git remote get-url origin *> $null
if ($LASTEXITCODE -ne 0) { Run-Native "git" @("remote", "add", "origin", "https://github.com/$full.git") }
Run-Native "git" @("push", "-u", "origin", "main")
$repoUrl = "https://github.com/$full"
Ok "Pushed to $repoUrl"
if ($Private) { Warn "Private repo: Render needs access. The API step may fail; the browser fallback will explain what to do." }

# ------------------------------------------------------------------ 5. Render
$serviceUrl = $ServerUrl.TrimEnd("/")
if (-not $SkipRender -and -not $serviceUrl) {
    Step "Deploying the server on Render (free plan)"
    Write-Host ""
    Write-Host "    You need two things (both free, no card):" -ForegroundColor White
    Write-Host "      1. Render API key  : dashboard.render.com > Account Settings > API Keys > Create" -ForegroundColor Gray
    Write-Host "      2. Upstash Redis   : console.upstash.com > Create database > copy the REST URL and REST TOKEN" -ForegroundColor Gray
    Write-Host "         (keeps accounts and balances safe while the free server sleeps)" -ForegroundColor Gray
    Write-Host ""

    $renderKey = $env:RENDER_API_KEY
    if (-not $renderKey) { $renderKey = Plain (Read-Host "    Render API key" -AsSecureString) }
    $upUrl = $env:UPSTASH_REDIS_REST_URL
    if (-not $upUrl) { $upUrl = (Read-Host "    Upstash REST URL (or press Enter to skip)").Trim() }
    $upTok = $env:UPSTASH_REDIS_REST_TOKEN
    if ($upUrl -and -not $upTok) { $upTok = Plain (Read-Host "    Upstash REST TOKEN" -AsSecureString) }
    if (-not $upUrl) { Warn "No Upstash database: data will reset every time the free server sleeps or redeploys." }

    $hdr = @{ Authorization = "Bearer $renderKey"; Accept = "application/json" }
    $created = $false
    try {
        $owners = Invoke-RestMethod -Uri "https://api.render.com/v1/owners?limit=20" -Headers $hdr -Method Get
        $list = @($owners)
        if ($list.Count -eq 0) { throw "No Render workspace found for this API key." }
        $pick = $list[0]
        if ($list.Count -gt 1) {
            for ($i = 0; $i -lt $list.Count; $i++) { Write-Host ("      [{0}] {1}" -f $i, $list[$i].owner.name) }
            $n = Read-Host "    Which workspace? (number)"
            $pick = $list[[int]$n]
        }
        $ownerId = $pick.owner.id
        Ok ("Render workspace: " + $pick.owner.name)

        $adminEmail = "admin@techet.pay"
        $chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
        $adminPass = -join (1..16 | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })
        $env2 = @(
            @{ key = "TECHET_ADMIN_EMAIL";    value = $adminEmail },
            @{ key = "TECHET_ADMIN_PASSWORD"; value = $adminPass }
        )
        if ($upUrl) {
            $env2 += @{ key = "UPSTASH_REDIS_REST_URL";   value = $upUrl }
            $env2 += @{ key = "UPSTASH_REDIS_REST_TOKEN"; value = $upTok }
        }

        $name = $ServiceName
        for ($attempt = 1; $attempt -le 3 -and -not $created; $attempt++) {
            $body = @{
                type              = "web_service"
                name              = $name
                ownerId           = $ownerId
                repo              = $repoUrl
                branch            = "main"
                autoDeployTrigger = "commit"
                envVars           = $env2
                serviceDetails    = @{
                    runtime            = "docker"
                    plan               = "free"
                    region             = $Region
                    healthCheckPath    = "/healthz"
                    envSpecificDetails = @{ dockerfilePath = "./server/Dockerfile"; dockerContext = "./server" }
                }
            } | ConvertTo-Json -Depth 10
            try {
                $resp = Invoke-RestMethod -Uri "https://api.render.com/v1/services" -Headers $hdr -Method Post -Body $body -ContentType "application/json"
                $svc = $resp.service
                if (-not $svc) { $svc = $resp }
                $serviceUrl = $svc.serviceDetails.url
                if (-not $serviceUrl) { $serviceUrl = "https://$name.onrender.com" }
                $created = $true
                Ok "Render service created: $name"
            } catch {
                $msg = Get-ErrBody $_
                if ($msg -match "(?i)already|taken|unique|exists") {
                    $name = "$ServiceName-" + (-join ((48..57) + (97..102) | Get-Random -Count 4 | ForEach-Object { [char]$_ }))
                    Note "Name is taken, trying $name"
                } else { throw $msg }
            }
        }
        if (-not $created) { throw "Could not find a free service name." }
    } catch {
        $adminPass = $null   # the blueprint generates its own admin password instead
        Warn "Automatic Render setup did not work:"
        Note ($_ | Out-String).Trim()
        Write-Host ""
        Write-Host "    Fallback: one click in the browser." -ForegroundColor White
        Write-Host "      1. Click 'Apply' on the page that opens (Render reads render.yaml from your repo)" -ForegroundColor Gray
        Write-Host "      2. Paste the two Upstash values when asked" -ForegroundColor Gray
        Write-Host "      3. Wait until the service says Live, then copy its address (https://....onrender.com)" -ForegroundColor Gray
        Start-Process "https://render.com/deploy?repo=$repoUrl"
        $serviceUrl = (Read-Host "    Paste the Render service address").Trim().TrimEnd("/")
        if (-not $serviceUrl) { Die "No server address given." }
    }
    $renderKey = $null; $upTok = $null

    Step "Waiting for the server to come online (first build takes 3-6 minutes)"
    $live = $false
    $deadline = (Get-Date).AddMinutes(15)
    while ((Get-Date) -lt $deadline) {
        try {
            $r = Invoke-WebRequest -Uri "$serviceUrl/healthz" -UseBasicParsing -TimeoutSec 25
            if ($r.StatusCode -eq 200 -and $r.Content -match "ok") { $live = $true; break }
        } catch {}
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 15
    }
    Write-Host ""
    if ($live) { Ok "Server is live: $serviceUrl" }
    else { Warn "Still building. Check dashboard.render.com. Continuing; the app will work once it is live." }
}

# ------------------------------------------------------------------ 6. APK
$apkPath = $null
if (-not $SkipApk) {
    Step "Building the Android APK on GitHub Actions (about 5 minutes)"
    if (-not $serviceUrl) { $serviceUrl = "https://$ServiceName.onrender.com"; Warn "No server address known, using $serviceUrl" }
    gh variable set SERVER_URL --body $serviceUrl --repo $full | Out-Null

    $dispatched = $false
    for ($i = 1; $i -le 8 -and -not $dispatched; $i++) {
        gh workflow run android.yml --repo $full --ref main -f server_url=$serviceUrl *> $null
        if ($LASTEXITCODE -eq 0) { $dispatched = $true } else { Start-Sleep -Seconds 5 }
    }
    if (-not $dispatched) { Die "Could not start the APK build. Open $repoUrl/actions and run 'Build Android APK' manually." }

    $runId = $null
    for ($i = 1; $i -le 20 -and -not $runId; $i++) {
        Start-Sleep -Seconds 4
        $runId = (gh run list --workflow android.yml --repo $full --event workflow_dispatch --limit 1 --json databaseId --jq ".[0].databaseId" | Out-String).Trim()
    }
    if (-not $runId) { Die "The build did not start. Check $repoUrl/actions" }
    gh run watch $runId --repo $full --exit-status
    if ($LASTEXITCODE -ne 0) { Die "The APK build failed. Open $repoUrl/actions/runs/$runId and send me the red error." }

    gh release download apk-latest --repo $full --pattern "TechET-Pay.apk" --dir "$PSScriptRoot" --clobber
    if ($LASTEXITCODE -eq 0) { $apkPath = Join-Path $PSScriptRoot "TechET-Pay.apk"; Ok "APK saved: $apkPath" }
    else { Warn "Build finished but the download failed. Get it from $repoUrl/releases/tag/apk-latest" }
}

# ------------------------------------------------------------------ done
Write-Host ""
Write-Host "=====================================================" -ForegroundColor Green
Write-Host "  TechET Pay is deployed" -ForegroundColor Green
Write-Host "=====================================================" -ForegroundColor Green
Write-Host "  GitHub repo : $repoUrl"
if ($serviceUrl) {
    Write-Host "  Dashboard   : $serviceUrl/dashboard"
    Write-Host "  Admin panel : $serviceUrl/admin"
    if ($adminPass) { Write-Host "  Admin login : $adminEmail  /  $adminPass   (save this)" -ForegroundColor Yellow }
    else { Write-Host "  Admin login : see TECHET_ADMIN_PASSWORD in the Render service > Environment" -ForegroundColor Yellow }
    Write-Host "  API base    : $serviceUrl/v1"
}
if ($apkPath) {
    Write-Host "  Android APK : $apkPath"
    Write-Host "    Copy it to your phone and open it (allow 'install unknown apps' once)."
}
Write-Host ""
Write-Host "  Free Render servers sleep after 15 idle minutes; the first request then takes about a minute."
Write-Host "  To move to a real server later: docker compose up -d --build   (see README.md)"
Write-Host ""
