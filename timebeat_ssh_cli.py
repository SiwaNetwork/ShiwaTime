#!/usr/bin/env python3
"""
Timebeat SSH CLI Interface
Предоставляет SSH интерфейс для управления и мониторинга timebeat
"""

import asyncio
import asyncssh
import yaml
import json
import subprocess
import sys
import os
import logging
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Any

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('/var/log/timebeat/timebeat_ssh_cli.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

class TimebeatCLI:
    """Класс для управления timebeat через CLI команды"""
    
    def __init__(self, config_path: str = "/etc/timebeat/timebeat.yml"):
        self.config_path = config_path
        self.config = None
        self.load_config()
        
    def load_config(self) -> bool:
        """Загрузка конфигурации timebeat"""
        try:
            with open(self.config_path, 'r') as f:
                self.config = yaml.safe_load(f)
            return True
        except Exception as e:
            logger.error(f"Failed to load config: {e}")
            return False
    
    def save_config(self) -> bool:
        """Сохранение конфигурации timebeat"""
        try:
            with open(self.config_path, 'w') as f:
                yaml.dump(self.config, f, default_flow_style=False)
            return True
        except Exception as e:
            logger.error(f"Failed to save config: {e}")
            return False
    
    def get_status(self) -> str:
        """Получение статуса timebeat"""
        try:
            result = subprocess.run(['systemctl', 'status', 'timebeat'], 
                                 capture_output=True, text=True)
            return result.stdout
        except Exception as e:
            return f"Error getting status: {e}"
    
    def start_service(self) -> str:
        """Запуск сервиса timebeat"""
        try:
            result = subprocess.run(['systemctl', 'start', 'timebeat'], 
                                 capture_output=True, text=True)
            if result.returncode == 0:
                return "Timebeat service started successfully"
            else:
                return f"Failed to start timebeat: {result.stderr}"
        except Exception as e:
            return f"Error starting service: {e}"
    
    def stop_service(self) -> str:
        """Остановка сервиса timebeat"""
        try:
            result = subprocess.run(['systemctl', 'stop', 'timebeat'], 
                                 capture_output=True, text=True)
            if result.returncode == 0:
                return "Timebeat service stopped successfully"
            else:
                return f"Failed to stop timebeat: {result.stderr}"
        except Exception as e:
            return f"Error stopping service: {e}"
    
    def restart_service(self) -> str:
        """Перезапуск сервиса timebeat"""
        try:
            result = subprocess.run(['systemctl', 'restart', 'timebeat'], 
                                 capture_output=True, text=True)
            if result.returncode == 0:
                return "Timebeat service restarted successfully"
            else:
                return f"Failed to restart timebeat: {result.stderr}"
        except Exception as e:
            return f"Error restarting service: {e}"
    
    def get_logs(self, lines: int = 50) -> str:
        """Получение логов timebeat"""
        try:
            result = subprocess.run(['journalctl', '-u', 'timebeat', '-n', str(lines)], 
                                 capture_output=True, text=True)
            return result.stdout
        except Exception as e:
            return f"Error getting logs: {e}"
    
    def get_clock_sync_status(self) -> str:
        """Получение статуса синхронизации часов"""
        try:
            result = subprocess.run(['chrony', 'sources'], capture_output=True, text=True)
            clock_info = result.stdout + "\n"
            
            result = subprocess.run(['timedatectl', 'status'], capture_output=True, text=True)
            clock_info += result.stdout
            
            return clock_info
        except Exception as e:
            return f"Error getting clock sync status: {e}"
    
    def list_protocols(self) -> str:
        """Список активных протоколов"""
        if not self.config:
            return "Configuration not loaded"
        
        protocols = []
        try:
            primary_clocks = self.config.get('timebeat', {}).get('clock_sync', {}).get('primary_clocks', [])
            secondary_clocks = self.config.get('timebeat', {}).get('clock_sync', {}).get('secondary_clocks', [])
            
            protocols.append("=== PRIMARY CLOCKS ===")
            for clock in primary_clocks:
                protocol = clock.get('protocol', 'unknown')
                disabled = clock.get('disable', False)
                status = "DISABLED" if disabled else "ENABLED"
                interface = clock.get('interface', clock.get('ip', clock.get('device', 'N/A')))
                protocols.append(f"  {protocol.upper()}: {status} ({interface})")
            
            protocols.append("\n=== SECONDARY CLOCKS ===")
            for clock in secondary_clocks:
                protocol = clock.get('protocol', 'unknown')
                disabled = clock.get('disable', False)
                status = "DISABLED" if disabled else "ENABLED"
                interface = clock.get('interface', clock.get('ip', clock.get('device', 'N/A')))
                protocols.append(f"  {protocol.upper()}: {status} ({interface})")
            
            return "\n".join(protocols)
        except Exception as e:
            return f"Error listing protocols: {e}"
    
    def toggle_protocol(self, protocol: str, clock_type: str = "primary") -> str:
        """Включение/выключение протокола"""
        if not self.config:
            return "Configuration not loaded"
        
        try:
            clock_list_key = f"{clock_type}_clocks"
            clocks = self.config.get('timebeat', {}).get('clock_sync', {}).get(clock_list_key, [])
            
            for clock in clocks:
                if clock.get('protocol', '').lower() == protocol.lower():
                    current_status = clock.get('disable', False)
                    clock['disable'] = not current_status
                    
                    if self.save_config():
                        new_status = "DISABLED" if not current_status else "ENABLED"
                        return f"Protocol {protocol.upper()} {new_status} in {clock_type} clocks"
                    else:
                        return "Failed to save configuration"
            
            return f"Protocol {protocol} not found in {clock_type} clocks"
        except Exception as e:
            return f"Error toggling protocol: {e}"

class TimebeatSSHServer:
    """SSH сервер для timebeat CLI"""
    
    def __init__(self, port: int = 2222):
        self.port = port
        self.cli = TimebeatCLI()
        self.commands = {
            'help': self.show_help,
            'status': self.show_status,
            'start': self.start_service,
            'stop': self.stop_service,
            'restart': self.restart_service,
            'logs': self.show_logs,
            'protocols': self.list_protocols,
            'enable': self.enable_protocol,
            'disable': self.disable_protocol,
            'clock': self.clock_status,
            'config': self.show_config,
            'reload': self.reload_config,
            'exit': self.exit_session,
            'quit': self.exit_session
        }
    
    async def handle_client(self, process):
        """Обработчик SSH клиента"""
        process.stdout.write("Welcome to Timebeat SSH CLI Interface\n")
        process.stdout.write("Type 'help' for available commands\n\n")
        
        while True:
            try:
                process.stdout.write("timebeat> ")
                line = await process.stdin.readline()
                if not line:
                    break
                
                command_line = line.strip()
                if not command_line:
                    continue
                
                parts = command_line.split()
                command = parts[0].lower()
                args = parts[1:]
                
                if command in self.commands:
                    try:
                        result = await self.commands[command](args)
                        process.stdout.write(result + "\n\n")
                    except Exception as e:
                        process.stdout.write(f"Error executing command: {e}\n\n")
                else:
                    process.stdout.write(f"Unknown command: {command}\n")
                    process.stdout.write("Type 'help' for available commands\n\n")
                    
            except asyncssh.BreakReceived:
                break
            except Exception as e:
                logger.error(f"Error in SSH session: {e}")
                break
        
        process.exit(0)
    
    async def show_help(self, args: List[str]) -> str:
        """Показать справку по командам"""
        help_text = """
Available Commands:
------------------
status          - Show timebeat service status
start           - Start timebeat service
stop            - Stop timebeat service  
restart         - Restart timebeat service
logs [lines]    - Show timebeat logs (default: 50 lines)
protocols       - List all configured protocols
enable <proto>  - Enable a protocol (ptp, ntp, pps, nmea, phc)
disable <proto> - Disable a protocol
clock           - Show clock synchronization status
config          - Show current configuration
reload          - Reload configuration from file
help            - Show this help message
exit/quit       - Exit the session

Examples:
---------
timebeat> status
timebeat> logs 100
timebeat> enable ptp
timebeat> disable nmea
timebeat> protocols
        """
        return help_text
    
    async def show_status(self, args: List[str]) -> str:
        """Показать статус сервиса"""
        return self.cli.get_status()
    
    async def start_service(self, args: List[str]) -> str:
        """Запустить сервис"""
        return self.cli.start_service()
    
    async def stop_service(self, args: List[str]) -> str:
        """Остановить сервис"""
        return self.cli.stop_service()
    
    async def restart_service(self, args: List[str]) -> str:
        """Перезапустить сервис"""
        return self.cli.restart_service()
    
    async def show_logs(self, args: List[str]) -> str:
        """Показать логи"""
        lines = 50
        if args and args[0].isdigit():
            lines = int(args[0])
        return self.cli.get_logs(lines)
    
    async def list_protocols(self, args: List[str]) -> str:
        """Список протоколов"""
        return self.cli.list_protocols()
    
    async def enable_protocol(self, args: List[str]) -> str:
        """Включить протокол"""
        if not args:
            return "Usage: enable <protocol>"
        
        protocol = args[0]
        clock_type = args[1] if len(args) > 1 else "primary"
        
        # Сначала включаем протокол
        result = self.cli.toggle_protocol(protocol, clock_type)
        if "ENABLED" in result:
            # Затем перезапускаем сервис
            restart_result = self.cli.restart_service()
            return f"{result}\n{restart_result}"
        return result
    
    async def disable_protocol(self, args: List[str]) -> str:
        """Отключить протокол"""
        if not args:
            return "Usage: disable <protocol>"
        
        protocol = args[0]
        clock_type = args[1] if len(args) > 1 else "primary"
        
        # Сначала отключаем протокол
        result = self.cli.toggle_protocol(protocol, clock_type)
        if "DISABLED" in result:
            # Затем перезапускаем сервис
            restart_result = self.cli.restart_service()
            return f"{result}\n{restart_result}"
        return result
    
    async def clock_status(self, args: List[str]) -> str:
        """Статус синхронизации часов"""
        return self.cli.get_clock_sync_status()
    
    async def show_config(self, args: List[str]) -> str:
        """Показать конфигурацию"""
        if self.cli.config:
            return yaml.dump(self.cli.config, default_flow_style=False)
        return "Configuration not loaded"
    
    async def reload_config(self, args: List[str]) -> str:
        """Перезагрузить конфигурацию"""
        if self.cli.load_config():
            return "Configuration reloaded successfully"
        return "Failed to reload configuration"
    
    async def exit_session(self, args: List[str]) -> str:
        """Выйти из сессии"""
        return "Goodbye!"

    async def start_server(self):
        """Запуск SSH сервера"""
        try:
            await asyncssh.listen(
                host='',
                port=self.port,
                server_host_keys=['ssh_host_key'],
                process_factory=self.handle_client,
                username='timebeat',
                password='timebeat123',  # В production использовать ключи
            )
            logger.info(f"Timebeat SSH CLI Server started on port {self.port}")
        except Exception as e:
            logger.error(f"Failed to start SSH server: {e}")
            raise

async def main():
    """Главная функция"""
    server = TimebeatSSHServer()
    
    try:
        await server.start_server()
        print(f"Timebeat SSH CLI Server running on port {server.port}")
        print("Connect with: ssh timebeat@localhost -p 2222")
        print("Password: timebeat123")
        
        # Держим сервер запущенным
        await asyncio.Event().wait()
        
    except KeyboardInterrupt:
        print("\nServer stopped by user")
    except Exception as e:
        print(f"Server error: {e}")

if __name__ == "__main__":
    asyncio.run(main())