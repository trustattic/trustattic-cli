@echo off
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/trustattic/trustattic-cli/master/install.ps1 | iex"
