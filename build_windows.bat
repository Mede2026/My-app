@echo off
REM Construit dist\CryptoBulle.exe. A lancer dans une invite de commandes Windows.
setlocal

echo [1/4] Environnement virtuel...
if not exist .venv (python -m venv .venv || goto :error)

echo [2/4] Dependances...
call .venv\Scripts\python.exe -m pip install --upgrade pip >nul
call .venv\Scripts\python.exe -m pip install -r requirements-dev.txt || goto :error

echo [3/4] Icone...
call .venv\Scripts\python.exe scripts\make_icon.py || goto :error

echo [4/4] Compilation...
call .venv\Scripts\python.exe -m PyInstaller --noconfirm --clean CryptoBulle.spec || goto :error

echo.
echo Termine : dist\CryptoBulle.exe
goto :eof

:error
echo.
echo La construction a echoue.
exit /b 1
