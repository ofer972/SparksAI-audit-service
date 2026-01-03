@echo off
setlocal enabledelayedexpansion

set OUTPUT_FILE=merged_go_files.txt
set SEARCH_DIR=.

echo Merging all Go files from %SEARCH_DIR%...
echo.

if exist "%OUTPUT_FILE%" del "%OUTPUT_FILE%"

for /r "%SEARCH_DIR%" %%f in (*.go) do (
    echo ======================================== >> "%OUTPUT_FILE%"
    echo File: %%f >> "%OUTPUT_FILE%"
    echo ======================================== >> "%OUTPUT_FILE%"
    echo. >> "%OUTPUT_FILE%"
    type "%%f" >> "%OUTPUT_FILE%"
    echo. >> "%OUTPUT_FILE%"
    echo. >> "%OUTPUT_FILE%"
)

echo Done! Merged files saved to: %OUTPUT_FILE%

