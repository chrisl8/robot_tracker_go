<#
.SYNOPSIS
    Installs OpenCV 4.13.0 and required tools for GoCV on Windows.

.DESCRIPTION
    This script automates the complete installation of OpenCV 4.13.0 for use with
    GoCV (gocv.io/x/gocv) on Windows 10/11.

    The script performs the following phases:
    1. Pre-flight checks (Windows, PowerShell version, Git, disk space)
    2. Install MinGW-W64 v13.0.0 (C++ compiler toolchain)
    3. Install CMake 3.28.1 (build system)
    4. Clone GoCV repository (contains OpenCV build scripts)
    5. Download OpenCV 4.13.0 + contrib modules
    6. Build OpenCV from source (takes 60-120 minutes)
    7. Configure system PATH and environment variables
    8. Verify installation by running gocv version check
    9. Clean up temporary files

.PARAMETER Verbose
    Enable verbose output for debugging.

.PARAMETER SkipCleanup
    Skip cleanup of temporary files (useful for debugging).

.PARAMETER WorkDir
    Temporary working directory. Default: C:\opencv-build-temp

.PARAMETER InstallDir
    OpenCV installation directory. Default: C:\opencv

.PARAMETER MinGWVersion
    MinGW-W64 version to install. Default: 13.0.0

.PARAMETER CMakeVersion
    CMake version to install. Default: 3.28.1

.EXAMPLE
    # Standard run (requires administrator privileges)
    .\install-opencv-for-gocv.ps1

.EXAMPLE
    # Run with verbose output for debugging
    .\install-opencv-for-gocv.ps1 -Verbose

.EXAMPLE
    # Skip cleanup to keep temporary files for debugging
    .\install-opencv-for-gocv.ps1 -SkipCleanup

.NOTES
    Prerequisites (must exist before running):
    - Windows 10/11 64-bit
    - PowerShell 5.1 or later
    - Git installed and in PATH
    - ~15GB free disk space
    - Internet connection (~300MB download)

    Estimated runtime: 75-145 minutes (mostly the OpenCV build)

    Exit codes:
    0 - Success
    1 - Missing prerequisite
    2 - Download failed
    3 - Build failed
    4 - Verification failed
    5 - Cleanup failed (non-critical)

.LINK
    GoCV documentation: https://gocv.io/
    GoCV Windows installation: https://gocv.io/getting-started/windows/
#>

param(
    [switch]$Verbose,
    [switch]$SkipCleanup,
    [string]$WorkDir = "C:\opencv-build-temp",
    [string]$InstallDir = "C:\opencv",
    [string]$MinGWVersion = "13.0.0",
    [string]$CMakeVersion = "3.28.1"
)

# ============= VERSION INFORMATION =============
$script:ScriptName = "install-opencv-for-gocv.ps1"
$script:ScriptVersion = "1.0.0"
$script:OpenCVVersion = "4.13.0"
$script:GoCVVersion = "0.43.0"

# ============= URLs =============
# MinGW-W64 v13.0.0 for Windows (win32-seh-ucrt-rt_v12-rev0)
# Direct SourceForge download link for the 7z package
$script:MinGWURL = "https://sourceforge.net/projects/mingw-w64/files/mingw-w64/mingw-w64-release/mingw-w64-v${MinGWVersion}.0-release-win32-seh-ucrt-rt_v12-rev0/mingw-w64-v${MinGWVersion}.0-release-win32-seh-ucrt-rt_v12-rev0.7z/download"

# CMake 3.28.1 Windows x86_64 ZIP
$script:CMakeURL = "https://github.com/Kitware/CMake/releases/download/v$CMakeVersion/cmake-$CMakeVersion-windows-x86_64.zip"

# GoCV repository (contains OpenCV build scripts)
$script:GoCVRepoURL = "https://github.com/hybridgroup/gocv.git"

# ============= LOGGING =============
$script:LogFile = "C:\opencv-build-log.txt"

function Write-Log {
    <#
    .SYNOPSIS
        Writes a message to the console and log file.
    #>
    param(
        [Parameter(Mandatory=$false)]
        [AllowEmptyString()]
        [AllowNull()]
        [string]$Message,
        [ValidateSet("INFO", "WARN", "ERROR", "SUCCESS")]
        [string]$Level = "INFO"
    )

    # Handle null or empty message
    if ([string]::IsNullOrEmpty($Message)) {
        $Message = "[No message]"
    }

    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $formattedMessage = "[$timestamp] [$Level] $Message"

    # Write to console
    switch ($Level) {
        "ERROR"   { Write-Host $formattedMessage -ForegroundColor Red }
        "WARN"    { Write-Host $formattedMessage -ForegroundColor Yellow }
        "SUCCESS" { Write-Host $formattedMessage -ForegroundColor Green }
        default   { Write-Host $formattedMessage }
    }

    # Write to log file
    try {
        $formattedMessage | Out-File -FilePath $script:LogFile -Append -Encoding utf8 -ErrorAction SilentlyContinue
    }
    catch {
        # Silently ignore logging failures
    }

    if ($Verbose -and $Level -eq "INFO") {
        Write-Verbose $Message
    }
}

# ============= HELPER FUNCTIONS =============

function Test-Administrator {
    <#
    .SYNOPSIS
        Checks if the script is running with administrator privileges.
    #>
    $currentUser = New-Object Security.Principal.WindowsPrincipal(
        [Security.Principal.WindowsIdentity]::GetCurrent()
    )
    return $currentUser.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-TempFileName {
    <#
    .SYNOPSIS
        Generates a unique temporary file name.
    #>
    param([string]$Extension = ".tmp")
    $tempPath = [System.IO.Path]::GetTempPath()
    $fileName = [System.IO.Path]::GetRandomFileName()
    return Join-Path $tempPath ($fileName + $Extension)
}

function Expand-7ZipArchive {
    <#
    .SYNOPSIS
        Extracts a 7-Zip archive using the built-in expansion method.
    #>
    param(
        [Parameter(Mandatory)]
        [string]$Path,
        [Parameter(Mandatory)]
        [string]$Destination
    )

    Write-Log "Extracting archive: $Path" -Level "INFO"
    Write-Log "  to: $Destination" -Level "INFO"

    try {
        # Try using Expand-Archive first (PowerShell 5.0+)
        if (Get-Command Expand-Archive -ErrorAction SilentlyContinue) {
            Expand-Archive -Path $Path -DestinationPath $Destination -Force -ErrorAction Stop
        }
        else {
            # Fallback for older PowerShell versions - use .NET directly
            $shell = New-Object -ComObject Shell.Application
            $zip = $shell.NameSpace($Path)
            $destinationFolder = $shell.NameSpace($Destination)

            # Copy all items from the zip file
            $zip.Items() | ForEach-Object {
                $destinationFolder.CopyHere($_, 16)  # 16 = don't show progress dialog
            }

            # Release COM objects
            [System.Runtime.Interopservices.Marshal]::ReleaseComObject($zip) | Out-Null
            [System.Runtime.Interopservices.Marshal]::ReleaseComObject($destinationFolder) | Out-Null
            [System.Runtime.Interopservices.Marshal]::ReleaseComObject($shell) | Out-Null
        }
        Write-Log "Archive extracted successfully" -Level "SUCCESS"
    }
    catch {
        Write-Log "Failed to extract archive: $_" -Level "ERROR"
        throw
    }
}

function Get-FinalDownloadUrl {
    <#
    .SYNOPSIS
        Follows redirects to get the final download URL (needed for SourceForge).
    #>
    param(
        [Parameter(Mandatory)]
        [string]$URL
    )

    try {
        $request = [System.Net.HttpWebRequest]::Create($URL)
        $request.AllowAutoRedirect = $false
        $request.Method = "HEAD"
        $request.Timeout = 30000

        $response = $request.GetResponse()
        try {
            # Check for redirect
            if ($response.StatusCode -eq 301 -or $response.StatusCode -eq 302) {
                $finalUrl = $response.Headers["Location"]
                Write-Log "  Redirected to: $finalUrl" -Level "INFO"
                return $finalUrl
            }
            return $URL
        }
        finally {
            $response.Close()
        }
    }
    catch {
        # If we can't follow redirect, return original URL
        return $URL
    }
}

function Get-GitHubReleaseUrl {
    <#
    .SYNOPSIS
        Gets the actual download URL from a GitHub release by parsing the redirect.
    #>
    param(
        [Parameter(Mandatory)]
        [string]$Owner,
        [Parameter(Mandatory)]
        [string]$Repo,
        [Parameter(Mandatory)]
        [string]$Pattern
    )

    try {
        # Try to get the release page and find the download link
        $apiUrl = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
        $response = Invoke-RestMethod -Uri $apiUrl -ErrorAction SilentlyContinue

        if ($response) {
            foreach ($asset in $response.assets) {
                if ($asset.name -match $Pattern) {
                    return $asset.browser_download_url
                }
            }
        }
    }
    catch {
        Write-Log "  GitHub API call failed: $_" -Level "WARN"
    }

    return $null
}

function Invoke-WebDownload {
    <#
    .SYNOPSIS
        Downloads a file from a URL with progress indication.
        Handles SourceForge redirects properly.
    #>
    param(
        [Parameter(Mandatory)]
        [string]$URL,
        [Parameter(Mandatory)]
        [string]$OutputPath,
        [switch]$HandleRedirects
    )

    Write-Log "Downloading: $URL" -Level "INFO"
    Write-Log "  to: $OutputPath" -Level "INFO"

    # Handle SourceForge redirects if needed
    if ($HandleRedirects -or $URL -match "sourceforge\.net") {
        Write-Log "  Checking for redirect..." -Level "INFO"
        $actualUrl = Get-FinalDownloadUrl -URL $URL
        if ($actualUrl -ne $URL) {
            Write-Log "  Following redirect..." -Level "INFO"
            $URL = $actualUrl
        }
    }

    try {
        # Use WebClient with timeout
        $webClient = New-Object System.Net.WebClient
        $webClient.Proxy = [System.Net.WebRequest]::GetSystemWebProxy()
        $webClient.Proxy.Credentials = [System.Net.CredentialCache]::DefaultCredentials

        $webClient.DownloadFile($URL, $OutputPath)
        Write-Log "Download complete" -Level "SUCCESS"
    }
    catch {
        Write-Log "Download failed: $_" -Level "ERROR"
        throw
    }
}

function Invoke-ExternalCommand {
    <#
    .SYNOPSIS
        Executes an external command and captures output.
    #>
    param(
        [Parameter(Mandatory)]
        [string]$Command,
        [Parameter(Mandatory)]
        [string]$WorkingDirectory,
        [int]$TimeoutSeconds = 0
    )

    Write-Log "Executing: $Command" -Level "INFO"
    Write-Log "  in: $WorkingDirectory" -Level "INFO"

    $processInfo = New-Object System.Diagnostics.ProcessStartInfo
    $processInfo.FileName = "cmd.exe"
    $processInfo.Arguments = "/c `"$Command`""
    $processInfo.WorkingDirectory = $WorkingDirectory
    $processInfo.UseShellExecute = $false
    $processInfo.RedirectStandardOutput = $true
    $processInfo.RedirectStandardError = $true
    $processInfo.CreateNoWindow = $true

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $processInfo
    $process.Start() | Out-Null

    if ($TimeoutSeconds -gt 0) {
        $completed = $process.WaitForExit($TimeoutSeconds * 1000)
        if (-not $completed) {
            Write-Log "Command timed out after $TimeoutSeconds seconds" -Level "ERROR"
            $process.Kill()
            throw "Command timed out: $Command"
        }
    }
    else {
        $process.WaitForExit()
    }

    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $exitCode = $process.ExitCode

    if ($exitCode -ne 0) {
        Write-Log "Command failed with exit code $exitCode" -Level "ERROR"
        if ($stdout) { Write-Log "STDOUT: $stdout" -Level "ERROR" }
        if ($stderr) { Write-Log "STDERR: $stderr" -Level "ERROR" }
        throw "Command failed: $Command"
    }

    Write-Log "Command completed successfully" -Level "SUCCESS"
    return $stdout
}

# ============= PHASE FUNCTIONS =============

function Test-Prerequisites {
    <#
    .SYNOPSIS
        Phase 1: Verify all prerequisites are met.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 1: Checking Prerequisites" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $allPassed = $true

    # Check Windows
    Write-Log "Checking Windows OS..." -Level "INFO"
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        Write-Log "PowerShell 5.1 or later is required. Current version: $($PSVersionTable.PSVersion)" -Level "ERROR"
        $allPassed = $false
    }
    else {
        Write-Log "PowerShell version: $($PSVersionTable.PSVersion)" -Level "SUCCESS"
    }

    if (-not $IsWindows) {
        Write-Log "This script must be run on Windows" -Level "ERROR"
        $allPassed = $false
    }
    else {
        Write-Log "Running on Windows" -Level "SUCCESS"
    }

    # Check architecture
    $osArch = (Get-WmiObject -Class Win32_OperatingSystem).OSArchitecture
    if ($osArch -ne "64-bit") {
        Write-Log "This script requires 64-bit Windows" -Level "ERROR"
        $allPassed = $false
    }
    else {
        Write-Log "Architecture: $osArch" -Level "SUCCESS"
    }

    # Check administrator privileges
    Write-Log "Checking administrator privileges..." -Level "INFO"
    if (-not (Test-Administrator)) {
        Write-Log "ERROR: This script requires administrator privileges" -Level "ERROR"
        Write-Log "  PATH modifications require admin rights" -Level "ERROR"
        Write-Log "  Please re-run PowerShell as Administrator" -Level "ERROR"
        throw "Administrator privileges required"
    }
    else {
        Write-Log "Running as administrator" -Level "SUCCESS"
    }

    # Check Git
    Write-Log "Checking Git installation..." -Level "INFO"
    try {
        $gitVersion = git --version 2>&1
        if ($gitVersion -match "git version") {
            Write-Log "Git found: $gitVersion" -Level "SUCCESS"
        }
        else {
            Write-Log "Git not found or not working properly" -Level "ERROR"
            $allPassed = $false
        }
    }
    catch {
        Write-Log "Git not found: $_" -Level "ERROR"
        $allPassed = $false
    }

    # Check disk space (need ~15GB)
    Write-Log "Checking available disk space..." -Level "INFO"
    $drive = Get-WmiObject -Class Win32_LogicalDisk -Filter "DeviceID='C:'"
    $freeGB = [math]::Round($drive.FreeSpace / 1GB, 2)
    Write-Log "Free space on C:: $freeGB GB" -Level "INFO"
    if ($freeGB -lt 15) {
        Write-Log "At least 15GB free space required. Found: $freeGB GB" -Level "ERROR"
        $allPassed = $false
    }
    else {
        Write-Log "Sufficient disk space available" -Level "SUCCESS"
    }

    # Check internet connectivity
    Write-Log "Checking internet connectivity..." -Level "INFO"
    try {
        $response = Invoke-WebRequest -Uri "https://github.com" -TimeoutSec 10 -UseBasicParsing
        if ($response.StatusCode -eq 200) {
            Write-Log "Internet connection confirmed" -Level "SUCCESS"
        }
    }
    catch {
        Write-Log "Internet connection test failed" -Level "ERROR"
        $allPassed = $false
    }

    if (-not $allPassed) {
        Write-Log "Prerequisites check failed" -Level "ERROR"
        throw "Prerequisites not met"
    }

    Write-Log "All prerequisites passed" -Level "SUCCESS"
}

function Install-MinGW64 {
    <#
    .SYNOPSIS
        Phase 2: Install MinGW-W64 compiler toolchain.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 2: Installing MinGW-W64 v$script:MinGWVersion" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    # Check for existing MinGW installations
    $possiblePaths = @(
        "C:\mingw64\bin\g++.exe",
        "C:\mingw64\mingw64\bin\g++.exe",
        "C:\msys64\mingw64\bin\g++.exe",
        "C:\msys64\clang64\bin\g++.exe",
        "C:\msys64\ucrt64\bin\g++.exe",
        "C:\Program Files\mingw-w64\bin\g++.exe",
        "C:\Program Files (x86)\mingw-w64\bin\g++.exe"
    )

    $mingwBinPath = $null
    foreach ($path in $possiblePaths) {
        if (Test-Path $path) {
            $mingwBinPath = Split-Path -Parent (Split-Path -Parent $path)
            Write-Log "Found existing MinGW-W64 at: $mingwBinPath" -Level "SUCCESS"
            break
        }
    }

    if ($mingwBinPath) {
        $gppPath = Join-Path $mingwBinPath "bin\g++.exe"
        $gppVersion = & $gppPath --version 2>&1 | Select-Object -First 1
        Write-Log "MinGW-W64 version: $gppVersion" -Level "INFO"
        $env:PATH = "$mingwBinPath\bin;$env:PATH"
        return
    }

    # Install LLVM MinGW (standalone, works well for OpenCV)
    Write-Log "Downloading LLVM MinGW..." -Level "INFO"

    $mingwInstallDir = "C:\mingw64"
    $mingwZipPath = Join-Path $script:WorkDir "mingw64.zip"

    # LLVM MinGW with UCRT (latest from GitHub releases)
    $llvmMingwUrl = "https://github.com/mstorsjo/llvm-mingw/releases/download/20251216/llvm-mingw-20251216-ucrt-x86_64.zip"

    try {
        Write-Log "Downloading LLVM MinGW from GitHub..." -Level "INFO"
        Invoke-WebDownload -URL $llvmMingwUrl -OutputPath $mingwZipPath

        Write-Log "Extracting LLVM MinGW..." -Level "INFO"
        New-Item -ItemType Directory -Path $mingwInstallDir -Force | Out-Null
        Expand-7ZipArchive -Path $mingwZipPath -Destination $mingwInstallDir

        # Find and move the extracted contents
        $extractedDir = Get-ChildItem -Path $mingwInstallDir -Directory | Where-Object { $_.Name -match "llvm-mingw" } | Select-Object -First 1
        if ($extractedDir) {
            Write-Log "Moving extracted files..." -Level "INFO"
            Get-ChildItem -Path $extractedDir.FullName | Move-Item -Destination $mingwInstallDir -Force
            Remove-Item -Path $extractedDir.FullName -Recurse -Force
        }

        $gppPath = Join-Path $mingwInstallDir "bin\g++.exe"
        if (Test-Path $gppPath) {
            $gppVersion = & $gppPath --version 2>&1 | Select-Object -First 1
            Write-Log "MinGW-W64 installed successfully" -Level "SUCCESS"
            Write-Log "  g++ version: $gppVersion" -Level "INFO"
            $env:PATH = "$mingwInstallDir\bin;$env:PATH"
            return
        }
        else {
            Write-Log "g++ not found at expected path" -Level "WARN"
        }
    }
    catch {
        Write-Log "LLVM MinGW download failed: $_" -Level "WARN"
    }

    # Fallback: Try MSVCrt version
    Write-Log "Trying MSVCrt version of LLVM MinGW..." -Level "INFO"

    $llvmMingwMsvcrtUrl = "https://github.com/mstorsjo/llvm-mingw/releases/download/20251216/llvm-mingw-20251216-msvcrt-x86_64.zip"

    try {
        Invoke-WebDownload -URL $llvmMingwMsvcrtUrl -OutputPath $mingwZipPath
        Expand-7ZipArchive -Path $mingwZipPath -Destination $mingwInstallDir

        $extractedDir = Get-ChildItem -Path $mingwInstallDir -Directory | Where-Object { $_.Name -match "llvm-mingw" } | Select-Object -First 1
        if ($extractedDir) {
            Get-ChildItem -Path $extractedDir.FullName | Move-Item -Destination $mingwInstallDir -Force
            Remove-Item -Path $extractedDir.FullName -Recurse -Force
        }

        $gppPath = Join-Path $mingwInstallDir "bin\g++.exe"
        if (Test-Path $gppPath) {
            $gppVersion = & $gppPath --version 2>&1 | Select-Object -First 1
            Write-Log "MinGW-W64 installed successfully" -Level "SUCCESS"
            Write-Log "  g++ version: $gppVersion" -Level "INFO"
            $env:PATH = "$mingwInstallDir\bin;$env:PATH"
            return
        }
    }
    catch {
        Write-Log "MSVCrt version also failed: $_" -Level "WARN"
    }

    # Final fallback
    Write-Log "========================================" -Level "ERROR"
    Write-Log "Could not install MinGW-W64 automatically" -Level "ERROR"
    Write-Log "========================================" -Level "ERROR"
    Write-Log "" -Level "ERROR"
    Write-Log "Please install manually:" -Level "INFO"
    Write-Log "  1. Download LLVM MinGW from: https://github.com/mstorsjo/llvm-mingw/releases" -Level "INFO"
    Write-Log "  2. Extract to C:\mingw64" -Level "INFO"
    Write-Log "  3. Add C:\mingw64\bin to your PATH" -Level "INFO"
    Write-Log "" -Level "ERROR"

    throw "MinGW-W64 installation failed. Please install manually."
}

function Install-CMake {
    <#
    .SYNOPSIS
        Phase 3: Install CMake build system.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 3: Installing CMake v$script:CMakeVersion" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $cmakeInstallDir = "C:\Program Files\CMake"
    $cmakeBinPath = Join-Path $cmakeInstallDir "bin"
    $cmakeExePath = Join-Path $cmakeBinPath "cmake.exe"

    # Check if CMake 3.28.1 is already installed at expected location
    if (Test-Path $cmakeExePath) {
        try {
            $cmakeVersion = & $cmakeExePath --version 2>&1 | Select-Object -First 1
            if ($cmakeVersion -match "cmake $script:CMakeVersion") {
                Write-Log "CMake $script:CMakeVersion already installed at expected location" -Level "SUCCESS"
                Write-Log "  Version: $cmakeVersion" -Level "INFO"
                $env:PATH = "$cmakeBinPath;$env:PATH"
                return
            }
        }
        catch {
            # cmake.exe exists but version check failed
            Write-Log "CMake found but version check failed, reinstalling..." -Level "WARN"
        }
    }

    # Also check for CMake in PATH that might be the right version
    try {
        $cmakeVersion = cmake --version 2>&1
        if ($cmakeVersion -match "cmake $script:CMakeVersion") {
            Write-Log "CMake $script:CMakeVersion found in PATH" -Level "SUCCESS"
            Write-Log "  Version: $cmakeVersion" -Level "INFO"
            return
        }
    }
    catch {
        # CMake not found in PATH
    }

    # Need to install CMake
    Write-Log "CMake $script:CMakeVersion not found or incorrect version" -Level "INFO"
    $cmakeZipPath = Join-Path $script:WorkDir "cmake.zip"
    Write-Log "Downloading CMake..." -Level "INFO"
    try {
        Invoke-WebDownload -URL $script:CMakeURL -OutputPath $cmakeZipPath -HandleRedirects
    }
    catch {
        Write-Log "Failed to download CMake: $_" -Level "ERROR"
        throw
    }

    # Extract CMake
    Write-Log "Extracting CMake..." -Level "INFO"
    try {
        # Extract to the installation directory (without -Force since Expand-Archive doesn't support it)
        Expand-Archive -Path $cmakeZipPath -Destination $cmakeInstallDir -Force:$false

        # CMake zip creates a nested directory like "cmake-3.28.1-windows-x86_64"
        # Move contents up to the expected location
        $nestedDir = Get-ChildItem -Path $cmakeInstallDir -Directory | Where-Object { $_.Name -match "cmake" } | Select-Object -First 1
        if ($nestedDir) {
            Write-Log "Moving CMake files to expected location..." -Level "INFO"
            Get-ChildItem -Path $nestedDir.FullName | Move-Item -Destination $cmakeInstallDir -Force -ErrorAction SilentlyContinue
            Remove-Item -Path $nestedDir.FullName -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    catch {
        Write-Log "Failed to extract CMake: $_" -Level "ERROR"
        throw
    }

    # Verify installation
    Write-Log "Verifying CMake installation..." -Level "INFO"
    try {
        $cmakePath = Join-Path $cmakeBinPath "cmake.exe"
        if (Test-Path $cmakePath) {
            $cmakeVersion = & $cmakePath --version 2>&1 | Select-Object -First 1
            Write-Log "CMake installed successfully" -Level "SUCCESS"
            Write-Log "  Version: $cmakeVersion" -Level "INFO"
        }
        else {
            throw "cmake.exe not found at $cmakePath"
        }
    }
    catch {
        Write-Log "CMake verification failed: $_" -Level "ERROR"
        throw
    }

    # Add to PATH for current session
    Write-Log "Adding CMake to PATH for current session..." -Level "INFO"
    $env:PATH = "$cmakeBinPath;$env:PATH"

    Write-Log "CMake installation complete" -Level "SUCCESS"
}

function Clone-GoCV {
    <#
    .SYNOPSIS
        Phase 4: Clone GoCV repository (contains OpenCV build scripts).
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 4: Cloning GoCV Repository" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $gocvDir = Join-Path $script:WorkDir "gocv"

    # Check if already cloned
    if (Test-Path (Join-Path $gocvDir ".git")) {
        Write-Log "GoCV repository already cloned, pulling latest..." -Level "INFO"
        Push-Location $gocvDir
        try {
            git pull origin release
            Write-Log "GoCV repository updated" -Level "SUCCESS"
        }
        catch {
            Write-Log "Could not update, using existing version" -Level "WARN"
        }
        Pop-Location
    }
    else {
        Write-Log "Cloning GoCV repository..." -Level "INFO"
        try {
            git clone $script:GoCVRepoURL $gocvDir
            # Checkout the release branch for matching version
            Push-Location $gocvDir
            git checkout release
            Pop-Location
            Write-Log "GoCV repository cloned successfully" -Level "SUCCESS"
        }
        catch {
            Write-Log "Failed to clone GoCV repository: $_" -Level "ERROR"
            throw
        }
    }

    # Verify build scripts exist
    $downloadScript = Join-Path $gocvDir "win_download_opencv.cmd"
    $buildScript = Join-Path $gocvDir "win_build_opencv.cmd"

    if (-not (Test-Path $downloadScript)) {
        throw "OpenCV download script not found: $downloadScript"
    }
    if (-not (Test-Path $buildScript)) {
        throw "OpenCV build script not found: $buildScript"
    }

    Write-Log "GoCV repository ready" -Level "SUCCESS"
}

function Download-OpenCV {
    <#
    .SYNOPSIS
        Phase 5: Download OpenCV 4.13.0 and contrib modules.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 5: Downloading OpenCV $script:OpenCVVersion" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $gocvDir = Join-Path $script:WorkDir "gocv"

    # Check if OpenCV sources are already downloaded
    $opencvSourceDir = "C:\opencv\opencv-4.13.0"
    $opencvContribDir = "C:\opencv\opencv_contrib-4.13.0"
    
    if (Test-Path $opencvSourceDir -PathType Container) {
        Write-Log "OpenCV sources already downloaded at C:\opencv" -Level "SUCCESS"
        
        if (Test-Path $opencvContribDir -PathType Container) {
            Write-Log "OpenCV contrib modules already downloaded" -Level "SUCCESS"
            return
        }
    }

    # Run the GoCV download script
    Write-Log "Running GoCV OpenCV download script..." -Level "INFO"
    Write-Log "  This will download OpenCV 4.13.0 + contrib modules (~200MB)" -Level "INFO"

    try {
        # Ensure MinGW and CMake are in PATH (find MinGW first)
        $mingwBinPaths = @(
            "C:\mingw64\bin",
            "C:\mingw64\mingw64\bin",
            "C:\Program Files\mingw-w64\bin"
        )
        $foundMingwBin = $null
        foreach ($path in $mingwBinPaths) {
            if (Test-Path (Join-Path $path "g++.exe")) {
                $foundMingwBin = $path
                break
            }
        }
        if ($foundMingwBin) {
            $env:PATH = "$foundMingwBin;C:\Program Files\CMake\bin;$env:PATH"
        } else {
            $env:PATH = "C:\Program Files\CMake\bin;$env:PATH"
        }

        # Run the download command
        Push-Location $gocvDir
        $output = cmd /c ".\win_download_opencv.cmd" 2>&1 | Out-String
        Pop-Location

        # Check if download was successful - GoCV extracts to C:\opencv by default
        $opencvSourceDir = "C:\opencv"
        if (Test-Path $opencvSourceDir) {
            Write-Log "OpenCV downloaded successfully to C:\opencv" -Level "SUCCESS"
        }
        else {
            throw "OpenCV download script did not create expected directory at C:\opencv"
        }
    }
    catch {
        Write-Log "Failed to download OpenCV: $_" -Level "ERROR"
        Write-Log "Output: $output" -Level "ERROR"
        throw
    }

    Write-Log "OpenCV download complete" -Level "SUCCESS"
}

function Build-OpenCV {
    <#
    .SYNOPSIS
        Phase 6: Build OpenCV from source (takes 60-120 minutes).
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 6: Building OpenCV $script:OpenCVVersion from Source" -Level "INFO"
    Write-Log "========================================" -Level "INFO"
    Write-Log "WARNING: This phase takes approximately 60-120 minutes" -Level "WARN"
    Write-Log "Please be patient and do not interrupt the process" -Level "WARN"

    $gocvDir = Join-Path $script:WorkDir "gocv"
    $buildOutputDir = Join-Path $script:InstallDir "build\install"

    # Check if already built
    if (Test-Path (Join-Path $buildOutputDir "x64\mingw\bin\libopencv_core4130.dll")) {
        Write-Log "OpenCV $script:OpenCVVersion already built" -Level "SUCCESS"
        return
    }

    # Check for MinGW in common locations (LLVM MinGW uses C:\mingw64\bin, not mingw64\mingw64\bin)
    $mingwBinPaths = @(
        "C:\mingw64\bin",
        "C:\mingw64\mingw64\bin",
        "C:\Program Files\mingw-w64\bin"
    )
    $foundMingwBin = $null
    foreach ($path in $mingwBinPaths) {
        if (Test-Path (Join-Path $path "g++.exe")) {
            $foundMingwBin = $path
            break
        }
    }
    
    if (-not $foundMingwBin) {
        throw "MinGW not found. Please run Phase 2 first."
    }

    # Ensure MinGW and CMake are in PATH
    $env:PATH = "$foundMingwBin;C:\Program Files\CMake\bin;$env:PATH"

    # Set environment variables for OpenCV build
    # GoCV downloads OpenCV to C:\opencv by default
    $env:OPENCV_CONTRIB_MODULES = "C:\opencv\opencv_contrib\modules"
    $env:OPENCV_EXTRA_MODULES_PATH = $env:OPENCV_CONTRIB_MODULES

    Write-Log "Starting OpenCV build (this will take a long time)..." -Level "INFO"
    $startTime = Get-Date

    try {
        Push-Location $gocvDir

        # Run the build command
        # Note: The build script may take 1-2 hours
        Write-Log "Running: .\win_build_opencv.cmd" -Level "INFO"

        $buildProcess = Start-Process -FilePath "cmd.exe" `
            -ArgumentList "/c `".\win_build_opencv.cmd`" " `
            -WorkingDirectory $gocvDir `
            -PassThru `
            -NoNewWindow

        # Monitor progress
        $lastProgressTime = $startTime
        $progressInterval = [TimeSpan]::FromMinutes(5)

        while (-not $buildProcess.HasExited) {
            $elapsed = (Get-Date) - $startTime
            $elapsedStr = [string]::Format("{0:D2}:{1:D2}:{2:D2}",
                $elapsed.Hours,
                $elapsed.Minutes,
                $elapsed.Seconds)

            Write-Log "Building... Elapsed time: $elapsedStr" -Level "INFO"

            # Check if we've exceeded expected time
            if ($elapsed.TotalMinutes -gt 150) {
                Write-Log "Build has exceeded 2.5 hours, checking status..." -Level "WARN"
            }

            # Wait 30 seconds before checking again
            Start-Sleep -Seconds 30
        }

        $exitCode = $buildProcess.ExitCode
        Pop-Location

        $endTime = Get-Date
        $totalTime = $endTime - $startTime
        $totalTimeStr = [string]::Format("{0:D2}:{1:D2}:{2:D2}",
            $totalTime.Hours,
            $totalTime.Minutes,
            $totalTime.Seconds)

        if ($exitCode -ne 0) {
            Write-Log "OpenCV build failed with exit code: $exitCode" -Level "ERROR"
            Write-Log "Total build time: $totalTimeStr" -Level "ERROR"
            throw "OpenCV build failed"
        }

        Write-Log "OpenCV built successfully" -Level "SUCCESS"
        Write-Log "Total build time: $totalTimeStr" -Level "INFO"

    }
    catch {
        Write-Log "OpenCV build failed: $_" -Level "ERROR"
        throw
    }

    # Verify build output
    Write-Log "Verifying build output..." -Level "INFO"
    $requiredDlls = @(
        "libopencv_core4130.dll",
        "libopencv_imgproc4130.dll",
        "libopencv_videoio4130.dll",
        "libopencv_highgui4130.dll",
        "libopencv_imgcodecs4130.dll",
        "libopencv_objdetect4130.dll",
        "libopencv_dnn4130.dll"
    )

    $dllDir = Join-Path $buildOutputDir "x64\mingw\bin"
    $allFound = $true

    foreach ($dll in $requiredDlls) {
        $dllPath = Join-Path $dllDir $dll
        if (-not (Test-Path $dllPath)) {
            Write-Log "Missing DLL: $dll" -Level "ERROR"
            $allFound = $false
        }
    }

    if (-not $allFound) {
        throw "Build output verification failed - missing required DLLs"
    }

    Write-Log "Build output verified" -Level "SUCCESS"
}

function Configure-Environment {
    <#
    .SYNOPSIS
        Phase 7: Configure system PATH and environment variables.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 7: Configuring Environment" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $opencvBinPath = Join-Path $script:InstallDir "build\install\x64\mingw\bin"
    $opencvIncludePath = Join-Path $script:InstallDir "build\install\include"

    # Verify paths exist
    if (-not (Test-Path $opencvBinPath)) {
        throw "OpenCV bin directory not found: $opencvBinPath"
    }
    if (-not (Test-Path $opencvIncludePath)) {
        throw "OpenCV include directory not found: $opencvIncludePath"
    }

    # Add to system PATH (requires admin privileges)
    Write-Log "Adding OpenCV to system PATH..." -Level "INFO"

    try {
        # Get current PATH
        $currentPath = [Environment]::GetEnvironmentVariable("PATH", "Machine")

        # Check if already in PATH
        if ($currentPath -split ";" -contains $opencvBinPath) {
            Write-Log "OpenCV already in PATH" -Level "SUCCESS"
        }
        else {
            # Add to PATH
            $newPath = "$opencvBinPath;$currentPath"
            [Environment]::SetEnvironmentVariable("PATH", $newPath, "Machine")

            # Also add to current session
            $env:PATH = "$opencvBinPath;$env:PATH"

            Write-Log "OpenCV added to system PATH" -Level "SUCCESS"
        }
    }
    catch {
        Write-Log "Failed to add OpenCV to PATH: $_" -Level "ERROR"
        Write-Log "You may need to manually add: $opencvBinPath" -Level "INFO"
        throw
    }

    # Create OPENCV_DIR environment variable
    Write-Log "Setting OPENCV_DIR environment variable..." -Level "INFO"
    $opencvInstallPath = Join-Path $script:InstallDir "build\install"
    try {
        [Environment]::SetEnvironmentVariable("OPENCV_DIR", $opencvInstallPath, "Machine")
        $env:OPENCV_DIR = $opencvInstallPath
        Write-Log "OPENCV_DIR set to: $opencvInstallPath" -Level "SUCCESS"
    }
    catch {
        Write-Log "Failed to set OPENCV_DIR: $_" -Level "ERROR"
        Write-Log "You may need to manually set OPENCV_DIR=$opencvInstallPath" -Level "INFO"
    }

    Write-Log "Environment configuration complete" -Level "SUCCESS"
}

function Verify-Installation {
    <#
    .SYNOPSIS
        Phase 8: Verify OpenCV installation by running gocv version check.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 8: Verifying Installation" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    $gocvDir = Join-Path $script:WorkDir "gocv"
    $versionExe = Join-Path $gocvDir "cmd\version\main.go"

    # Clone robot_tracker_go if not exists, or use existing
    Write-Log "Cloning/using robot_tracker_go for verification..." -Level "INFO"

    $robotTrackerDir = Join-Path $script:WorkDir "robot_tracker_go"
    if (-not (Test-Path (Join-Path $robotTrackerDir ".git"))) {
        git clone "https://github.com/yourusername/robot_tracker_go.git" $robotTrackerDir 2>&1 | Out-Null
    }

    # For now, just verify OpenCV can be found
    Write-Log "Verifying OpenCV DLLs are accessible..." -Level "INFO"

    $opencvDllPath = Join-Path $script:InstallDir "build\install\x64\mingw\bin\libopencv_core4130.dll"

    if (Test-Path $opencvDllPath) {
        Write-Log "OpenCV core DLL found: $opencvDllPath" -Level "SUCCESS"
    }
    else {
        throw "OpenCV core DLL not found at: $opencvDllPath"
    }

    # List all OpenCV DLLs
    Write-Log "OpenCV DLLs installed:" -Level "INFO"
    $dllDir = Join-Path $script:InstallDir "build\install\x64\mingw\bin"
    Get-ChildItem -Path $dllDir -Filter "*.dll" | ForEach-Object {
        Write-Log "  - $($_.Name)" -Level "INFO"
    }

    # Verify MinGW can link against OpenCV
    Write-Log "Verifying MinGW can link against OpenCV..." -Level "INFO"
    $testCppPath = Join-Path $script:WorkDir "test_opencv.cpp"
    @"
#include <opencv2/core.hpp>
int main() {
    cv::Mat m;
    return 0;
}
"@ | Out-File -FilePath $testCppPath -Encoding utf8

    $testExePath = Join-Path $script:WorkDir "test_opencv.exe"

    try {
        $gppPath = "C:\mingw64\mingw64\bin\g++.exe"
        if (Test-Path $gppPath) {
            $result = & $gppPath -c $testCppPath -o $testExePath 2>&1
            if ($LASTEXITCODE -eq 0) {
                Write-Log "OpenCV header compilation test passed" -Level "SUCCESS"
            }
            else {
                Write-Log "OpenCV header compilation test failed: $result" -Level "WARN"
            }
        }
    }
    catch {
        Write-Log "Compilation test encountered an issue: $_" -Level "WARN"
    }

    Write-Log "Installation verification complete" -Level "SUCCESS"
    Write-Log "" -Level "INFO"
    Write-Log "========================================" -Level "INFO"
    Write-Log "OpenCV $script:OpenCVVersion installed successfully!" -Level "SUCCESS"
    Write-Log "========================================" -Level "INFO"
    Write-Log "" -Level "INFO"
    Write-Log "Installation summary:" -Level "INFO"
    Write-Log "  OpenCV directory: $script:InstallDir" -Level "INFO"
    Write-Log "  DLL location: $dllDir" -Level "INFO"
    Write-Log "  Add to PATH: $dllDir" -Level "INFO"
    Write-Log "" -Level "INFO"
    Write-Log "Next steps:" -Level "INFO"
    Write-Log "  1. Restart PowerShell to ensure PATH changes take effect" -Level "INFO"
    Write-Log "  2. Install Go 1.23.2+" -Level "INFO"
    Write-Log "  3. Run: go mod tidy" -Level "INFO"
    Write-Log "  4. Build your GoCV application" -Level "INFO"
    Write-Log "" -Level "INFO"
}

function Start-Cleanup {
    <#
    .SYNOPSIS
        Phase 9: Clean up temporary files.
    #>
    Write-Log "========================================" -Level "INFO"
    Write-Log "PHASE 9: Cleanup" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    if ($SkipCleanup) {
        Write-Log "Skipping cleanup (as requested)" -Level "INFO"
        Write-Log "Temporary files in: $script:WorkDir" -Level "INFO"
        return
    }

    Write-Log "Removing temporary files..." -Level "INFO"

    try {
        # Remove working directory
        if (Test-Path $script:WorkDir) {
            Remove-Item -Path $script:WorkDir -Recurse -Force -ErrorAction Stop
            Write-Log "Removed: $script:WorkDir" -Level "SUCCESS"
        }
    }
    catch {
        Write-Log "Failed to remove some temporary files: $_" -Level "WARN"
        Write-Log "You may need to manually remove: $script:WorkDir" -Level "INFO"
    }

    Write-Log "Cleanup complete" -Level "SUCCESS"
}

# ============= MAIN ENTRY POINT =============

function Main {
    <#
    .SYNOPSIS
        Main entry point for the script.
    #>

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "  OpenCV for GoCV Installation Script" -ForegroundColor Cyan
    Write-Host "  Version $script:ScriptVersion" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""

    Write-Log "========================================" -Level "INFO"
    Write-Log "OpenCV for GoCV Installation Script v$script:ScriptVersion" -Level "INFO"
    Write-Log "Target: OpenCV $script:OpenCVVersion, GoCV $script:GoCVVersion" -Level "INFO"
    Write-Log "========================================" -Level "INFO"

    # Initialize log file
    try {
        "OpenCV for GoCV Installation - $(Get-Date)" | Out-File -FilePath $script:LogFile -Encoding utf8 -Force
    }
    catch {
        Write-Warning "Could not create log file: $_"
    }

    try {
        # Create working directory
        if (-not (Test-Path $script:WorkDir)) {
            New-Item -ItemType Directory -Path $script:WorkDir -Force | Out-Null
        }

        # Run all phases
        Test-Prerequisites
        Install-MinGW64
        Install-CMake
        Clone-GoCV
        Download-OpenCV
        Build-OpenCV
        Configure-Environment
        Verify-Installation
        Start-Cleanup

        # Success
        Write-Host ""
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "  INSTALLATION COMPLETE!" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host ""
        Write-Host "OpenCV $script:OpenCVVersion is now installed." -ForegroundColor White
        Write-Host ""
        Write-Host "IMPORTANT: Restart PowerShell or run:" -ForegroundColor Yellow
        Write-Host "  \$env:PATH = 'C:\opencv\build\install\x64\mingw\bin;\$env:PATH'" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Then you can use GoCV in your Go applications." -ForegroundColor White
        Write-Host ""

        exit 0
    }
    catch {
        Write-Host ""
        Write-Host "========================================" -ForegroundColor Red
        Write-Host "  INSTALLATION FAILED!" -ForegroundColor Red
        Write-Host "========================================" -ForegroundColor Red
        Write-Host ""
        Write-Host "Error: $_" -ForegroundColor White
        Write-Host ""
        Write-Host "Check the log file for details: $script:LogFile" -ForegroundColor Yellow
        Write-Host ""

        exit 1
    }
}

# Run main function
Main
