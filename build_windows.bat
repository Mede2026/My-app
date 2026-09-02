@echo off
REM Construit CryptoBulle. Usage :
REM   build_windows.bat            -> dist\CryptoBulle\CryptoBulle.exe (demarrage le plus rapide)
REM   build_windows.bat onefile    -> dist\CryptoBulle.exe (fichier unique)
setlocal

if /i "%~1"=="onefile" set CRYPTOBULLE_ONEFILE=1

echo [1/4] Environnement virtuel...
if not exist .venv (python -m venv .venv || goto :error)

echo [2/4] Outils de construction...
call .venv\Scripts\python.exe -m pip install --upgrade pip >nul
call .venv\Scripts\python.exe -m pip install -r requirements-dev.txt || goto :error

echo [3/4] Icone...
call .venv\Scripts\python.exe scripts\make_icon.py || goto :error

echo [4/4] Compilation...
call .venv\Scripts\python.exe -m PyInstaller --noconfirm --clean CryptoBulle.spec || goto :error

echo.
if defined CRYPTOBULLE_ONEFILE (echo Termine : dist\CryptoBulle.exe) else (echo Termine : dist\CryptoBulle\CryptoBulle.exe)
goto :eof

:error
echo.
echo La construction a echoue.
exit /b 1
