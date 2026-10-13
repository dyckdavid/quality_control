@echo off
rem Run once on each shop PC (right-click, Run as administrator) so Windows trusts
rem Quality Control installers signed with the shop's own certificate.
rem Keep QualityControl-signing.cer in the same folder as this file.

net session >nul 2>&1
if errorlevel 1 (
    echo Please right-click this file and choose "Run as administrator".
    pause
    exit /b 1
)

set "CERT=%~dp0QualityControl-signing.cer"
if not exist "%CERT%" (
    echo Could not find QualityControl-signing.cer next to this file.
    pause
    exit /b 1
)

certutil -f -addstore Root "%CERT%" || goto failed
certutil -f -addstore TrustedPublisher "%CERT%" || goto failed
echo.
echo Done. This PC now trusts Quality Control installers from David Dyck.
pause
exit /b 0

:failed
echo Could not add the certificate. Windows said the error is above.
pause
exit /b 1
