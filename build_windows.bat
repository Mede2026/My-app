@echo off
REM Construit dist\CryptoBulle.exe. A lancer dans une invite de commandes Windows.
REM Prerequis : Go 1.24 ou plus recent (https://go.dev/dl/).
setlocal

where go >nul 2>&1 || (echo Go n'est pas installe. Voir https://go.dev/dl/ & exit /b 1)

if not exist dist mkdir dist

echo Compilation...
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist\CryptoBulle.exe .\cmd\cryptobulle || goto :error

echo.
echo Termine : dist\CryptoBulle.exe
echo Double-cliquez dessus, rien d'autre a installer.
goto :eof

:error
echo.
echo La construction a echoue.
exit /b 1
