package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"mysql-backup/internal/backup"
	"mysql-backup/internal/config"
	"mysql-backup/internal/google"
	"mysql-backup/internal/scheduler"
	"mysql-backup/internal/service"
	"mysql-backup/internal/ssh"
)

type Handler struct {
	config           *config.Config
	backupService    *backup.Service
	schedulerService *scheduler.Service
	serviceManager   *service.Manager
}

func NewHandler(cfg *config.Config, backupService *backup.Service, schedulerService *scheduler.Service, serviceManager *service.Manager) *Handler {
	return &Handler{
		config:           cfg,
		backupService:    backupService,
		schedulerService: schedulerService,
		serviceManager:   serviceManager,
	}
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="pt-BR">
<head>
   <meta charset="UTF-8">
   <meta name="viewport" content="width=device-width, initial-scale=1.0">
   <title>MySQL Backup System</title>
   <script src="https://cdn.tailwindcss.com"></script>
   <script src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js" defer></script>
   <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
   <script>
       // Theme management
       function initTheme() {
           const savedTheme = localStorage.getItem('theme') || 'light';
           if (savedTheme === 'dark') {
               document.documentElement.classList.add('dark');
           }
           return savedTheme;
       }
       
       function toggleTheme() {
           const isDark = document.documentElement.classList.contains('dark');
           if (isDark) {
               document.documentElement.classList.remove('dark');
               localStorage.setItem('theme', 'light');
           } else {
               document.documentElement.classList.add('dark');
               localStorage.setItem('theme', 'dark');
           }
       }
       
       // Initialize theme before page loads
       initTheme();
   </script>
   <style>
       /* Custom dark theme styles */
       .dark {
           color-scheme: dark;
       }
       
       /* Theme toggle switch */
       .theme-switch {
           position: relative;
           display: inline-block;
           width: 60px;
           height: 34px;
       }
       
       .theme-switch input {
           opacity: 0;
           width: 0;
           height: 0;
       }
       
       .slider {
           position: absolute;
           cursor: pointer;
           top: 0;
           left: 0;
           right: 0;
           bottom: 0;
           background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
           transition: .4s;
           border-radius: 34px;
           box-shadow: 0 2px 10px rgba(0,0,0,0.2);
       }
       
       .slider:before {
           position: absolute;
           content: "";
           height: 26px;
           width: 26px;
           left: 4px;
           bottom: 4px;
           background: linear-gradient(135deg, #ffeaa7 0%, #fab1a0 100%);
           transition: .4s;
           border-radius: 50%;
           box-shadow: 0 2px 5px rgba(0,0,0,0.2);
       }
       
       input:checked + .slider {
           background: linear-gradient(135deg, #2d3748 0%, #1a202c 100%);
       }
       
       input:checked + .slider:before {
           transform: translateX(26px);
           background: linear-gradient(135deg, #81c784 0%, #4fc3f7 100%);
       }
       
       /* Smooth transitions for theme changes */
       * {
           transition: background-color 0.3s ease, border-color 0.3s ease, color 0.3s ease;
       }
       
       /* Custom scrollbar for dark theme */
       .dark ::-webkit-scrollbar {
           width: 8px;
       }
       
       .dark ::-webkit-scrollbar-track {
           background: #2d3748;
       }
       
       .dark ::-webkit-scrollbar-thumb {
           background: #4a5568;
           border-radius: 4px;
       }
       
       .dark ::-webkit-scrollbar-thumb:hover {
           background: #718096;
       }
   </style>
</head>
<body class="bg-gray-50 dark:bg-gray-900 transition-colors duration-300">
   <div class="min-h-screen" x-data="backupApp()">
       <!-- Header -->
       <header class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700">
           <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
               <div class="flex justify-between items-center py-4">
                   <div class="flex items-center">
                       <i class="fas fa-database text-blue-600 dark:text-blue-400 text-2xl mr-3"></i>
                       <h1 class="text-2xl font-bold text-gray-900 dark:text-white">MySQL Backup System</h1>
                   </div>
                   <div class="flex items-center space-x-4">
                       <!-- Theme Toggle -->
                       <div class="flex items-center space-x-3">
                           <i class="fas fa-sun text-yellow-500"></i>
                           <label class="theme-switch">
                               <input type="checkbox" x-model="darkMode" @change="toggleTheme()">
                               <span class="slider"></span>
                           </label>
                           <i class="fas fa-moon text-blue-400"></i>
                       </div>
                       
                       <div class="flex items-center space-x-2">
                           <div :class="status.mysql ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200' : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'" 
                                class="px-2 py-1 rounded-full text-xs font-medium">
                               <i :class="status.mysql ? 'fas fa-check' : 'fas fa-times'" class="mr-1"></i>
                               MySQL
                           </div>
                           <div :class="status.google ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200' : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'" 
                                class="px-2 py-1 rounded-full text-xs font-medium">
                               <i :class="status.google ? 'fas fa-check' : 'fas fa-times'" class="mr-1"></i>
                               Google
                           </div>
                           <div :class="status.scheduler ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200' : 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200'" 
                                class="px-2 py-1 rounded-full text-xs font-medium">
                               <i :class="status.scheduler ? 'fas fa-play' : 'fas fa-pause'" class="mr-1"></i>
                               Scheduler
                           </div>
                       </div>
                   </div>
               </div>
           </div>
       </header>

       <!-- Main Content -->
       <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
           <!-- Tabs -->
           <div class="bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700">
               <div class="border-b border-gray-200 dark:border-gray-700">
                   <nav class="-mb-px flex">
                       <button @click="activeTab = 'dashboard'" 
                               :class="activeTab === 'dashboard' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-tachometer-alt mr-2"></i>Dashboard
                       </button>
                       <button @click="activeTab = 'machines'" 
                               :class="activeTab === 'machines' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-server mr-2"></i>Servidores
                       </button>
                       <button @click="activeTab = 'backup'" 
                               :class="activeTab === 'backup' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-download mr-2"></i>Backup Manual
                       </button>
                       <button @click="activeTab = 'scheduler'" 
                               :class="activeTab === 'scheduler' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-clock mr-2"></i>Agendamentos
                       </button>
                       <button @click="activeTab = 'config'" 
                               :class="activeTab === 'config' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-cog mr-2"></i>Configurações
                       </button>
                       <button @click="activeTab = 'logs'" 
                               :class="activeTab === 'logs' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'"
                               class="w-1/6 py-4 px-1 text-center border-b-2 font-medium text-sm transition-colors">
                           <i class="fas fa-list mr-2"></i>Logs
                       </button>
                   </nav>
               </div>

               <div class="p-6">
                   <!-- Dashboard Tab -->
                   <div x-show="activeTab === 'dashboard'">
                       <h2 class="text-xl font-semibold mb-6 text-gray-900 dark:text-white">Dashboard</h2>
                       
                       <!-- Status Cards -->
                       <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
                           <div class="bg-gradient-to-r from-blue-500 to-blue-600 dark:from-blue-600 dark:to-blue-700 rounded-lg shadow-lg p-6 text-white">
                               <div class="flex items-center">
                                   <i class="fas fa-server text-3xl mr-4"></i>
                                   <div>
                                       <h3 class="text-lg font-semibold">Servidores</h3>
                                       <p class="text-blue-100" x-text="machines.length + ' configurados'"></p>
                                       <p class="text-sm text-blue-200" x-text="getEnabledMachines().length + ' ativos'"></p>
                                   </div>
                               </div>
                           </div>
                           
                           <div class="bg-gradient-to-r from-green-500 to-green-600 dark:from-green-600 dark:to-green-700 rounded-lg shadow-lg p-6 text-white">
                               <div class="flex items-center">
                                   <i class="fab fa-google-drive text-3xl mr-4"></i>
                                   <div>
                                       <h3 class="text-lg font-semibold">Google Drive</h3>
                                       <p class="text-green-100" x-text="status.google ? 'Autenticado' : 'Não Autenticado'"></p>
                                       <p class="text-sm text-green-200">Upload automático (.sql.gz)</p>
                                   </div>
                               </div>
                           </div>
                           
                           <div class="bg-gradient-to-r from-purple-500 to-purple-600 dark:from-purple-600 dark:to-purple-700 rounded-lg shadow-lg p-6 text-white">
                               <div class="flex items-center">
                                   <i class="fas fa-clock text-3xl mr-4"></i>
                                   <div>
                                       <h3 class="text-lg font-semibold">Agendamentos</h3>
                                       <p class="text-purple-100" x-text="status.scheduler ? 'Ativo' : 'Inativo'"></p>
                                       <p class="text-sm text-purple-200" x-text="schedules.length + ' agendamentos'"></p>
                                   </div>
                               </div>
                           </div>

                           <div class="bg-gradient-to-r from-orange-500 to-orange-600 dark:from-orange-600 dark:to-orange-700 rounded-lg shadow-lg p-6 text-white">
                               <div class="flex items-center">
                                   <i class="fas fa-database text-3xl mr-4"></i>
                                   <div>
                                       <h3 class="text-lg font-semibold">Bancos</h3>
                                       <p class="text-orange-100" x-text="databases.length + ' disponíveis'"></p>
                                       <p class="text-sm text-orange-200">No servidor selecionado</p>
                                   </div>
                               </div>
                           </div>
                       </div>

                       <!-- Quick Actions -->
                       <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-6 border border-gray-200 dark:border-gray-600 shadow-lg">
                           <h3 class="text-lg font-semibold mb-4 text-gray-900 dark:text-white">Ações Rápidas</h3>
                           <div class="flex flex-wrap gap-4">
                               <button @click="activeTab = 'machines'" 
                                       class="bg-blue-500 hover:bg-blue-600 dark:bg-blue-600 dark:hover:bg-blue-700 text-white px-4 py-2 rounded-lg flex items-center transition-colors shadow-md">
                                   <i class="fas fa-server mr-2"></i>Gerenciar Servidores
                               </button>
                               <button @click="activeTab = 'backup'" 
                                       class="bg-green-500 hover:bg-green-600 dark:bg-green-600 dark:hover:bg-green-700 text-white px-4 py-2 rounded-lg flex items-center transition-colors shadow-md">
                                   <i class="fas fa-download mr-2"></i>Backup Manual
                               </button>
                               <button @click="activeTab = 'scheduler'" 
                                       class="bg-purple-500 hover:bg-purple-600 dark:bg-purple-600 dark:hover:bg-purple-700 text-white px-4 py-2 rounded-lg flex items-center transition-colors shadow-md">
                                   <i class="fas fa-plus mr-2"></i>Novo Agendamento
                               </button>
                               <button @click="toggleScheduler()" 
                                       :class="status.scheduler ? 'bg-red-500 hover:bg-red-600 dark:bg-red-600 dark:hover:bg-red-700' : 'bg-indigo-500 hover:bg-indigo-600 dark:bg-indigo-600 dark:hover:bg-indigo-700'"
                                       class="text-white px-4 py-2 rounded-lg flex items-center transition-colors shadow-md">
                                   <i :class="status.scheduler ? 'fas fa-stop' : 'fas fa-play'" class="mr-2"></i>
                                   <span x-text="status.scheduler ? 'Parar Scheduler' : 'Iniciar Scheduler'"></span>
                               </button>
                           </div>
                       </div>
                   </div>

                   <!-- Machines Tab -->
                   <div x-show="activeTab === 'machines'">
                       <div class="flex justify-between items-center mb-6">
                           <h2 class="text-xl font-semibold text-gray-900 dark:text-white">Servidores MySQL</h2>
                           <button @click="showMachineForm = true; editingMachine = null; resetMachineForm()"
                                   class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded-lg flex items-center transition-colors">
                               <i class="fas fa-plus mr-2"></i>Novo Servidor
                           </button>
                       </div>

                       <!-- Lista de Servidores -->
                       <div class="space-y-4 mb-8">
                           <template x-for="machine in machines" :key="machine.id">
                               <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 shadow-sm">
                                   <div class="flex justify-between items-start">
                                       <div class="flex-1">
                                           <div class="flex items-center mb-2">
                                               <h3 class="text-lg font-medium text-gray-900 dark:text-white" x-text="machine.name"></h3>
                                               <span :class="machine.type === 'local' ? 'bg-blue-100 text-blue-800' : 'bg-green-100 text-green-800'" 
                                                     class="ml-3 px-2 py-1 rounded-full text-xs font-medium">
                                                   <i :class="machine.type === 'local' ? 'fas fa-home' : 'fas fa-cloud'" class="mr-1"></i>
                                                   <span x-text="machine.type === 'local' ? 'Local' : 'Remoto'"></span>
                                               </span>
                                               <span :class="machine.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 dark:bg-gray-600 text-gray-800 dark:text-gray-200'" 
                                                     class="ml-2 px-2 py-1 rounded-full text-xs font-medium">
                                                   <i :class="machine.enabled ? 'fas fa-check' : 'fas fa-times'" class="mr-1"></i>
                                                   <span x-text="machine.enabled ? 'Ativo' : 'Inativo'"></span>
                                               </span>
                                           </div>
                                           <p class="text-gray-600 dark:text-gray-400 text-sm mb-2" x-text="machine.description"></p>
                                           <div class="flex flex-wrap gap-4 text-sm text-gray-500 dark:text-gray-400">
                                               <div>
                                                   <i class="fas fa-server mr-1"></i>
                                                   <span x-text="machine.mysql.host + ':' + machine.mysql.port"></span>
                                               </div>
                                               <div>
                                                   <i class="fas fa-user mr-1"></i>
                                                   <span x-text="machine.mysql.username"></span>
                                               </div>
                                               <div x-show="machine.type === 'remote' && machine.ssh">
                                                   <i class="fas fa-key mr-1"></i>
                                                   <span x-text="'SSH: ' + machine.ssh.host + ':' + machine.ssh.port"></span>
                                               </div>
                                           </div>
                                       </div>
                                       <div class="flex space-x-2">
                                           <button @click="editMachine(machine)"
                                                   class="text-blue-600 hover:text-blue-800 transition-colors">
                                               <i class="fas fa-edit"></i>
                                           </button>
                                           <button @click="testMachineConnection(machine.id)"
                                                   class="text-green-600 hover:text-green-800 transition-colors">
                                               <i class="fas fa-plug"></i>
                                           </button>
                                           <button @click="deleteMachine(machine.id)"
                                                   :disabled="machine.id === 'local'"
                                                   :class="machine.id === 'local' ? 'text-gray-400' : 'text-red-600 hover:text-red-800 transition-colors'">
                                               <i class="fas fa-trash"></i>
                                           </button>
                                       </div>
                                   </div>
                               </div>
                           </template>
                       </div>

                       <!-- Modal de Servidor -->
                       <div x-show="showMachineForm" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" 
                            x-transition:enter="transition ease-out duration-300" 
                            x-transition:enter-start="opacity-0" 
                            x-transition:enter-end="opacity-100">
                           <div class="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-2xl max-h-screen overflow-y-auto shadow-lg">
                               <div class="flex justify-between items-center mb-6">
                                   <h3 class="text-lg font-semibold text-gray-900 dark:text-white" x-text="editingMachine ? 'Editar Servidor' : 'Novo Servidor'"></h3>
                                   <button @click="showMachineForm = false" class="text-gray-400 hover:text-gray-600 transition-colors">
                                       <i class="fas fa-times"></i>
                                   </button>
                               </div>

                               <form @submit.prevent="saveMachine()" class="space-y-6">
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Nome do Servidor:</label>
                                       <input type="text" x-model="machineForm.name" required
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Descrição:</label>
                                       <textarea x-model="machineForm.description" rows="2"
                                                 class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors"></textarea>
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Tipo:</label>
                                       <select x-model="machineForm.type" @change="machineTypeChanged()"
                                               class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           <option value="local">Local</option>
                                           <option value="remote">Remoto (SSH)</option>
                                       </select>
                                   </div>

                                   <!-- MySQL Configuration -->
                                   <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50 dark:bg-gray-700">
                                       <h4 class="text-md font-medium mb-4 text-gray-900 dark:text-white">Configuração MySQL</h4>
                                       <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host:</label>
                                               <input type="text" x-model="machineForm.mysql.host" required
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Porta:</label>
                                               <input type="number" x-model="machineForm.mysql.port" required
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Usuário:</label>
                                               <input type="text" x-model="machineForm.mysql.username" required
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Senha:</label>
                                               <input type="password" x-model="machineForm.mysql.password" required
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                       </div>
                                   </div>

                                   <!-- SSH Configuration (only for remote) -->
                                   <div x-show="machineForm.type === 'remote'" class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-gray-50 dark:bg-gray-700">
                                       <h4 class="text-md font-medium mb-4 text-gray-900 dark:text-white">Configuração SSH</h4>
                                       <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Host SSH:</label>
                                               <input type="text" x-model="machineForm.ssh.host"
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           <div>
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Porta SSH:</label>
                                               <input type="number" x-model="machineForm.ssh.port" value="22"
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           <div class="md:col-span-2">
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Usuário SSH:</label>
                                               <input type="text" x-model="machineForm.ssh.username"
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                           
                                           <!-- SSH Authentication Method -->
                                           <div class="md:col-span-2">
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Método de Autenticação SSH:</label>
                                               <div class="space-y-3">
                                                   <label class="flex items-center text-gray-700 dark:text-gray-300">
                                                       <input type="radio" x-model="sshAuthMethod" value="key" class="mr-3 transition-colors">
                                                       <span>Chave Privada SSH</span>
                                                   </label>
                                                   <label class="flex items-center text-gray-700 dark:text-gray-300">
                                                       <input type="radio" x-model="sshAuthMethod" value="password" class="mr-3 transition-colors">
                                                       <span>Senha SSH</span>
                                                   </label>
                                               </div>
                                           </div>

                                           <!-- SSH Key Authentication -->
                                           <div x-show="sshAuthMethod === 'key'" class="md:col-span-2">
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Chave Privada SSH:</label>
                                               <textarea x-model="machineForm.ssh.private_key" rows="4" placeholder="-----BEGIN PRIVATE KEY-----"
                                                                 class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors"></textarea>
                                               <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Cole sua chave privada SSH aqui</p>
                                           </div>
                                           
                                           <div x-show="sshAuthMethod === 'key'" class="md:col-span-2">
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Passphrase da Chave (opcional):</label>
                                               <input type="password" x-model="machineForm.ssh.passphrase"
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>

                                           <!-- SSH Password Authentication -->
                                           <div x-show="sshAuthMethod === 'password'" class="md:col-span-2">
                                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Senha SSH:</label>
                                               <input type="password" x-model="machineForm.ssh.password"
                                                      class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           </div>
                                       </div>
                                   </div>

                                   <div class="flex items-center">
                                       <input type="checkbox" x-model="machineForm.enabled" class="mr-3 rounded transition-colors">
                                       <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Ativar servidor</label>
                                   </div>

                                   <!-- Test Connection Button -->
                                   <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 shadow-lg">
                                       <button type="button" @click="testMachineConfig()" 
                                               :disabled="testingConnection"
                                               class="w-full bg-yellow-500 hover:bg-yellow-600 disabled:bg-gray-400 text-white px-4 py-2 rounded-lg flex items-center justify-center transition-colors">
                                           <i :class="testingConnection ? 'fas fa-spinner fa-spin' : 'fas fa-plug'" class="mr-2"></i>
                                           <span x-text="testingConnection ? 'Testando Conexão...' : 'Testar Conexão'"></span>
                                       </button>
                                       <p class="text-xs text-gray-500 dark:text-gray-400 mt-2 text-center">Teste a conexão SSH e MySQL antes de salvar</p>
                                   </div>

                                   <div class="flex justify-end space-x-4">
                                       <button type="button" @click="showMachineForm = false" 
                                               class="px-4 py-2 text-gray-600 dark:text-gray-400 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors">
                                           Cancelar
                                       </button>
                                       <button type="submit" 
                                               class="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors">
                                           <i class="fas fa-save mr-2"></i>Salvar
                                       </button>
                                   </div>
                               </form>
                           </div>
                       </div>
                   </div>

                   <!-- Manual Backup Tab -->
                   <div x-show="activeTab === 'backup'">
                       <h2 class="text-xl font-semibold mb-6 text-gray-900 dark:text-white">Backup Manual de Bancos de Dados</h2>
                       
                       <div class="space-y-6">
                           <div class="bg-blue-50 border border-blue-200 rounded-lg p-4 text-gray-900 dark:text-white">
                               <div class="flex items-center">
                                   <i class="fas fa-info-circle text-blue-600 mr-2"></i>
                                   <p class="text-blue-800 text-sm">
                                       Selecione o servidor e os bancos de dados para backup completo. Os arquivos serão salvos como .sql.gz e enviados para o Google Drive.
                                   </p>
                               </div>
                           </div>

                           <div>
                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Selecionar Servidor:</label>
                               <select x-model="selectedMachineId" @change="loadDatabasesForMachine()"
                                       class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   <option value="">Selecione um servidor...</option>
                                   <template x-for="machine in getEnabledMachines()" :key="machine.id">
                                       <option :value="machine.id" x-text="machine.name + ' (' + machine.type + ')'"></option>
                                   </template>
                               </select>
                           </div>
                           
                           <div x-show="selectedMachineId">
                               <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Selecionar Bancos de Dados:</label>
                               <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 max-h-60 overflow-y-auto">
                                   <div class="space-y-2">
                                       <label class="flex items-center text-gray-700 dark:text-gray-300">
                                           <input type="checkbox" @change="toggleAllDatabases($event)" class="mr-3 rounded transition-colors">
                                           <span class="font-medium">Selecionar Todos</span>
                                       </label>
                                       <hr class="my-2 border-gray-200 dark:border-gray-600">
                                       <template x-for="database in databases" :key="database">
                                           <label class="flex items-center text-gray-700 dark:text-gray-300">
                                               <input type="checkbox" :value="database" x-model="selectedDatabases" class="mr-3 rounded transition-colors">
                                               <i class="fas fa-database text-blue-600 mr-2"></i>
                                               <span x-text="database"></span>
                                           </label>
                                       </template>
                                       <div x-show="databases.length === 0 && selectedMachineId" class="text-gray-500 dark:text-gray-400 text-center py-4">
                                           Nenhum banco de dados encontrado. Verifique a conexão.
                                       </div>
                                   </div>
                               </div>
                           </div>
                           
                           <div class="flex items-center space-x-4">
                               <button @click="createBackup()" 
                                       :disabled="selectedDatabases.length === 0 || backupInProgress || !selectedMachineId"
                                       class="bg-blue-500 hover:bg-blue-600 disabled:bg-gray-400 text-white font-medium py-2 px-6 rounded-lg flex items-center transition-colors">
                                   <i :class="backupInProgress ? 'fas fa-spinner fa-spin' : 'fas fa-download'" class="mr-2"></i>
                                   <span x-text="backupInProgress ? 'Criando Backup...' : 'Criar Backup'"></span>
                               </button>
                               <div x-show="selectedDatabases.length > 0" class="text-sm text-gray-600 dark:text-gray-400">
                                   <span x-text="selectedDatabases.length"></span> banco(s) selecionado(s)
                               </div>
                           </div>
                       </div>
                   </div>

                   <!-- Scheduler Tab -->
                   <div x-show="activeTab === 'scheduler'">
                       <div class="flex justify-between items-center mb-6">
                           <h2 class="text-xl font-semibold text-gray-900 dark:text-white">Agendamentos de Backup</h2>
                           <button @click="showScheduleForm = true; editingSchedule = null; resetScheduleForm()" 
                                   class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded-lg flex items-center transition-colors">
                               <i class="fas fa-plus mr-2"></i>Novo Agendamento
                           </button>
                       </div>

                       <!-- Lista de Agendamentos -->
                       <div class="space-y-4 mb-8">
                           <template x-for="schedule in schedules" :key="schedule.id">
                               <div class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-4 shadow-sm">
                                   <div class="flex justify-between items-start">
                                       <div class="flex-1">
                                           <div class="flex items-center mb-2">
                                               <h3 class="text-lg font-medium text-gray-900 dark:text-white" x-text="schedule.name"></h3>
                                               <span :class="schedule.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 dark:bg-gray-600 text-gray-800 dark:text-gray-200'" 
                                                     class="ml-3 px-2 py-1 rounded-full text-xs font-medium">
                                                   <i :class="schedule.enabled ? 'fas fa-play' : 'fas fa-pause'" class="mr-1"></i>
                                                   <span x-text="schedule.enabled ? 'Ativo' : 'Inativo'"></span>
                                               </span>
                                           </div>
                                           <p class="text-gray-600 dark:text-gray-400 text-sm mb-2" x-text="schedule.description"></p>
                                           <div class="flex flex-wrap gap-4 text-sm text-gray-500 dark:text-gray-400">
                                               <div>
                                                   <i class="fas fa-server mr-1"></i>
                                                   <span x-text="getMachineName(schedule.machine_id)"></span>
                                               </div>
                                               <div>
                                                   <i class="fas fa-database mr-1"></i>
                                                   <span x-text="schedule.databases.length + ' banco(s)'"></span>
                                               </div>
                                               <div>
                                                   <i class="fas fa-calendar mr-1"></i>
                                                   <span x-text="formatDaysOfWeek(schedule.days_of_week)"></span>
                                               </div>
                                               <div>
                                                   <i class="fas fa-clock mr-1"></i>
                                                   <span x-text="schedule.times.join(', ')"></span>
                                               </div>
                                           </div>
                                       </div>
                                       <div class="flex space-x-2">
                                           <button @click="editSchedule(schedule)" 
                                                   class="text-blue-600 hover:text-blue-800 transition-colors">
                                               <i class="fas fa-edit"></i>
                                           </button>
                                           <button @click="toggleScheduleEnabled(schedule)" 
                                                   :class="schedule.enabled ? 'text-red-600 hover:text-red-800' : 'text-green-600 hover:text-green-800'" class="transition-colors">
                                               <i :class="schedule.enabled ? 'fas fa-pause' : 'fas fa-play'"></i>
                                           </button>
                                           <button @click="deleteSchedule(schedule.id)" 
                                                   class="text-red-600 hover:text-red-800 transition-colors">
                                               <i class="fas fa-trash"></i>
                                           </button>
                                       </div>
                                   </div>
                               </div>
                           </template>
                           <div x-show="schedules.length === 0" class="text-center py-8 text-gray-500 dark:text-gray-400">
                               <i class="fas fa-calendar-times text-4xl mb-4"></i>
                               <p>Nenhum agendamento configurado</p>
                               <p class="text-sm">Clique em "Novo Agendamento" para começar</p>
                           </div>
                       </div>

                       <!-- Controles do Scheduler -->
                       <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 shadow-lg">
                           <div class="flex justify-between items-center">
                               <div>
                                   <h3 class="font-medium text-gray-900 dark:text-white">Controle do Scheduler</h3>
                                   <p class="text-sm text-gray-600 dark:text-gray-400">
                                       Status: <span :class="status.scheduler ? 'text-green-600' : 'text-red-600'" 
                                                    x-text="status.scheduler ? 'Executando' : 'Parado'"></span>
                                   </p>
                               </div>
                               <button @click="toggleScheduler()" 
                                       :class="status.scheduler ? 'bg-red-500 hover:bg-red-600' : 'bg-green-500 hover:bg-green-600'"
                                       class="text-white px-4 py-2 rounded-lg flex items-center transition-colors">
                                   <i :class="status.scheduler ? 'fas fa-stop' : 'fas fa-play'" class="mr-2"></i>
                                   <span x-text="status.scheduler ? 'Parar Scheduler' : 'Iniciar Scheduler'"></span>
                               </button>
                           </div>
                       </div>

                       <!-- Modal de Agendamento -->
                       <div x-show="showScheduleForm" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" 
                            x-transition:enter="transition ease-out duration-300" 
                            x-transition:enter-start="opacity-0" 
                            x-transition:enter-end="opacity-100">
                           <div class="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-2xl max-h-screen overflow-y-auto shadow-lg">
                               <div class="flex justify-between items-center mb-6">
                                   <h3 class="text-lg font-semibold text-gray-900 dark:text-white" x-text="editingSchedule ? 'Editar Agendamento' : 'Novo Agendamento'"></h3>
                                   <button @click="showScheduleForm = false" class="text-gray-400 hover:text-gray-600 transition-colors">
                                       <i class="fas fa-times"></i>
                                   </button>
                               </div>

                               <form @submit.prevent="saveSchedule()" class="space-y-6">
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Nome do Agendamento:</label>
                                       <input type="text" x-model="scheduleForm.name" required
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Descrição (opcional):</label>
                                       <textarea x-model="scheduleForm.description" rows="2"
                                                 class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors"></textarea>
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Servidor:</label>
                                       <select x-model="scheduleForm.machine_id" @change="loadDatabasesForSchedule()" required
                                               class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                           <option value="">Selecione um servidor...</option>
                                           <template x-for="machine in getEnabledMachines()" :key="machine.id">
                                               <option :value="machine.id" x-text="machine.name + ' (' + machine.type + ')'"></option>
                                           </template>
                                       </select>
                                   </div>

                                   <div x-show="scheduleForm.machine_id">
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Bancos de Dados:</label>
                                       <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 max-h-40 overflow-y-auto">
                                           <div class="space-y-2">
                                               <template x-for="database in scheduleDatabases" :key="database">
                                                   <label class="flex items-center text-gray-700 dark:text-gray-300">
                                                       <input type="checkbox" :value="database" x-model="scheduleForm.databases" class="mr-3 rounded transition-colors">
                                                       <i class="fas fa-database text-blue-600 mr-2"></i>
                                                       <span x-text="database"></span>
                                                   </label>
                                               </template>
                                           </div>
                                       </div>
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Dias da Semana:</label>
                                       <div class="grid grid-cols-7 gap-2">
                                           <template x-for="(day, index) in daysOfWeek" :key="index">
                                               <label class="flex flex-col items-center text-gray-700 dark:text-gray-300">
                                                   <input type="checkbox" :value="index" x-model="scheduleForm.daysOfWeek" class="mb-1 transition-colors">
                                                   <span class="text-xs" x-text="day"></span>
                                               </label>
                                           </template>
                                       </div>
                                   </div>

                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Horários:</label>
                                       <div class="space-y-2">
                                           <template x-for="(time, index) in scheduleForm.times" :key="index">
                                               <div class="flex items-center space-x-2">
                                                   <input type="time" x-model="scheduleForm.times[index]" required
                                                          class="border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                                   <button type="button" @click="removeTime(index)" 
                                                           class="text-red-600 hover:text-red-800 transition-colors">
                                                       <i class="fas fa-trash"></i>
                                                   </button>
                                               </div>
                                           </template>
                                           <button type="button" @click="addTime()" 
                                                   class="text-blue-600 hover:text-blue-800 text-sm flex items-center transition-colors">
                                               <i class="fas fa-plus mr-1"></i>Adicionar Horário
                                           </button>
                                       </div>
                                   </div>

                                   <div class="flex items-center">
                                       <input type="checkbox" x-model="scheduleForm.enabled" class="mr-3 rounded transition-colors">
                                       <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Ativar agendamento</label>
                                   </div>

                                   <div class="flex justify-end space-x-4">
                                       <button type="button" @click="showScheduleForm = false" 
                                               class="px-4 py-2 text-gray-600 dark:text-gray-400 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors">
                                           Cancelar
                                       </button>
                                       <button type="submit" 
                                               class="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors">
                                           <i class="fas fa-save mr-2"></i>Salvar
                                       </button>
                                   </div>
                               </form>
                           </div>
                       </div>
                   </div>

                   <!-- Configuration Tab -->
                   <div x-show="activeTab === 'config'">
                       <h2 class="text-xl font-semibold mb-6 text-gray-900 dark:text-white">Configurações</h2>
                       
                       <div class="space-y-8">
                           <!-- Google Configuration -->
                           <div class="border border-gray-200 dark:border-gray-700 rounded-lg p-6 bg-white dark:bg-gray-800 shadow-lg">
                               <h3 class="text-lg font-medium mb-4 flex items-center text-gray-900 dark:text-white">
                                   <i class="fab fa-google mr-2 text-red-600"></i>Configuração Google
                               </h3>
                               <div class="space-y-4">
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Client ID:</label>
                                       <input type="text" x-model="config.google.client_id" 
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Client Secret:</label>
                                       <input type="password" x-model="config.google.client_secret" 
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">ID da Planilha Google Sheets:</label>
                                       <input type="text" x-model="config.google.sheet_id" 
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>
                                   <div>
                                       <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">ID da Pasta Google Drive:</label>
                                       <input type="text" x-model="config.google.drive_folder" 
                                              class="w-full border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-white transition-colors">
                                   </div>
                                   <div class="flex space-x-4">
                                       <button @click="authenticateGoogle()" 
                                               class="bg-red-500 hover:bg-red-600 text-white px-4 py-2 rounded-lg transition-colors">
                                           <i class="fab fa-google mr-2"></i>Autenticar com Google
                                       </button>
                                       <div x-show="status.google" class="flex items-center text-green-600">
                                           <i class="fas fa-check-circle mr-2"></i>Autenticado
                                       </div>
                                   </div>
                               </div>
                           </div>

                           <div class="flex justify-end">
                               <button @click="saveConfig()" 
                                       class="bg-green-500 hover:bg-green-600 text-white font-medium py-2 px-6 rounded-lg transition-colors">
                                   <i class="fas fa-save mr-2"></i>Salvar Configurações
                               </button>
                           </div>
                       </div>
                   </div>

                   <!-- Logs Tab -->
                   <div x-show="activeTab === 'logs'">
                       <h2 class="text-xl font-semibold mb-6 text-gray-900 dark:text-white">Logs de Backup</h2>
                       
                       <div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-lg">
                           <div class="overflow-x-auto">
                               <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                                   <thead class="bg-gray-50 dark:bg-gray-700">
                                       <tr>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Data/Hora</th>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Servidor</th>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Banco</th>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Status</th>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Arquivo</th>
                                           <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Tamanho</th>
                                       </tr>
                                   </thead>
                                   <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                                       <template x-for="log in logs" :key="log.id">
                                           <tr>
                                               <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white" x-text="formatDate(log.timestamp)"></td>
                                               <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white" x-text="getMachineName(log.machine_id)"></td>
                                               <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white" x-text="log.table_name"></td>
                                               <td class="px-6 py-4 whitespace-nowrap">
                                                   <span :class="log.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'" 
                                                         class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full">
                                                       <span x-text="log.success ? 'Sucesso' : 'Erro'"></span>
                                                   </span>
                                               </td>
                                               <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white" x-text="log.file_name"></td>
                                               <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white" x-text="formatFileSize(log.file_size)"></td>
                                           </tr>
                                       </template>
                                   </tbody>
                               </table>
                           </div>
                       </div>
                   </div>
               </div>
           </div>
       </main>
   </div>

   <script>
       function backupApp() {
           return {
               activeTab: 'dashboard',
               machines: [],
               databases: [],
               scheduleDatabases: [],
               selectedDatabases: [],
               selectedMachineId: '',
               schedules: [],
               logs: [],
               backupInProgress: false,
               showScheduleForm: false,
               showMachineForm: false,
               editingSchedule: null,
               editingMachine: null,
               status: {
                   mysql: false,
                   google: false,
                   scheduler: false
               },
               config: {
                   google: { client_id: '', client_secret: '', sheet_id: '', drive_folder: '' }
               },
               scheduleForm: {
                   name: '',
                   description: '',
                   enabled: true,
                   machine_id: '',
                   databases: [],
                   daysOfWeek: [],
                   times: ['09:00']
               },
               machineForm: {
                   name: '',
                   description: '',
                   type: 'local',
                   enabled: true,
                   mysql: { host: 'localhost', port: 3306, username: '', password: '' },
                   ssh: { host: '', port: 22, username: '', password: '', private_key: '', passphrase: '' }
               },
               daysOfWeek: ['Dom', 'Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb'],
               sshAuthMethod: 'key',
               testingConnection: false,
               darkMode: localStorage.getItem('theme') === 'dark',

               async init() {
                   await this.loadConfig();
                   await this.loadMachines();
                   await this.loadDatabases();
                   await this.loadSchedules();
                   await this.loadLogs();
                   await this.checkStatus();
               },

               toggleTheme() {
                   this.darkMode = !this.darkMode;
                   if (this.darkMode) {
                       document.documentElement.classList.add('dark');
                       localStorage.setItem('theme', 'dark');
                   } else {
                       document.documentElement.classList.remove('dark');
                       localStorage.setItem('theme', 'light');
                   }
               },

               async loadConfig() {
                   try {
                       const response = await fetch('/api/config');
                       if (response.ok) {
                           const config = await response.json();
                           this.config = config;
                       }
                   } catch (error) {
                       console.error('Failed to load config:', error);
                   }
               },

               async loadMachines() {
                   try {
                       const response = await fetch('/api/machines');
                       if (response.ok) {
                           this.machines = await response.json();
                           // Set default selected machine to local if available
                           if (!this.selectedMachineId && this.machines.length > 0) {
                               const localMachine = this.machines.find(m => m.id === 'local');
                               if (localMachine) {
                                   this.selectedMachineId = 'local';
                               }
                           }
                       }
                   } catch (error) {
                       console.error('Failed to load machines:', error);
                   }
               },

               async loadDatabases() {
                   if (!this.selectedMachineId) return;
                   
                   try {
                       const response = await fetch('/api/machines/' + this.selectedMachineId + '/databases');
                       if (response.ok) {
                           this.databases = await response.json();
                           this.status.mysql = true;
                       }
                   } catch (error) {
                       console.error('Failed to load databases:', error);
                       this.status.mysql = false;
                   }
               },

               async loadDatabasesForMachine() {
                   await this.loadDatabases();
                   this.selectedDatabases = [];
               },

               async loadDatabasesForSchedule() {
                   if (!this.scheduleForm.machine_id) return;
                   
                   try {
                       const response = await fetch('/api/machines/' + this.scheduleForm.machine_id + '/databases');
                       if (response.ok) {
                           this.scheduleDatabases = await response.json();
                       }
                   } catch (error) {
                       console.error('Failed to load databases for schedule:', error);
                   }
               },

               async loadSchedules() {
                   try {
                       const response = await fetch('/api/schedules');
                       if (response.ok) {
                           this.schedules = await response.json();
                       }
                   } catch (error) {
                       console.error('Failed to load schedules:', error);
                   }
               },

               async loadLogs() {
                   try {
                       const response = await fetch('/api/backup/logs');
                       if (response.ok) {
                           this.logs = await response.json();
                       }
                   } catch (error) {
                       console.error('Failed to load logs:', error);
                   }
               },

               async checkStatus() {
                   try {
                       const googleResponse = await fetch('/api/auth/google/status');
                       if (googleResponse.ok) {
                           const googleStatus = await googleResponse.json();
                           this.status.google = googleStatus.authenticated;
                       }

                       const schedulerResponse = await fetch('/api/scheduler/status');
                       if (schedulerResponse.ok) {
                           const schedulerStatus = await schedulerResponse.json();
                           this.status.scheduler = schedulerStatus.running;
                       }
                   } catch (error) {
                       console.error('Failed to check status:', error);
                   }
               },

               getEnabledMachines() {
                   return this.machines.filter(m => m.enabled);
               },

               getMachineName(machineId) {
                   const machine = this.machines.find(m => m.id === machineId);
                   return machine ? machine.name : 'Desconhecido';
               },

               toggleAllDatabases(event) {
                   if (event.target.checked) {
                       this.selectedDatabases = [...this.databases];
                   } else {
                       this.selectedDatabases = [];
                   }
               },

               async createBackup() {
                   if (this.selectedDatabases.length === 0 || !this.selectedMachineId) return;
                   
                   this.backupInProgress = true;
                   try {
                       const response = await fetch('/api/machines/' + this.selectedMachineId + '/backup', {
                           method: 'POST',
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify({ databases: this.selectedDatabases })
                       });
                       
                       if (response.ok) {
                           const results = await response.json();
                           let successCount = results.filter(r => r.success).length;
                           alert('Backup concluído! ' + successCount + '/' + results.length + ' bancos com sucesso.');
                           await this.loadLogs();
                       } else {
                           alert('Falha no backup!');
                       }
                   } catch (error) {
                       console.error('Backup failed:', error);
                       alert('Falha no backup!');
                   } finally {
                       this.backupInProgress = false;
                   }
               },

               // Machine management
               resetMachineForm() {
                   this.machineForm = {
                       name: '',
                       description: '',
                       type: 'local',
                       enabled: true,
                       mysql: { host: 'localhost', port: 3306, username: '', password: '' },
                       ssh: { host: '', port: 22, username: '', password: '', private_key: '', passphrase: '' }
                   };
                   this.sshAuthMethod = 'key';
               },

               machineTypeChanged() {
                   if (this.machineForm.type === 'local') {
                       this.machineForm.mysql.host = 'localhost';
                   }
               },

               editMachine(machine) {
                   this.editingMachine = machine;
                   this.machineForm = {
                       name: machine.name,
                       description: machine.description || '',
                       type: machine.type,
                       enabled: machine.enabled,
                       mysql: { ...machine.mysql },
                       ssh: machine.ssh ? { ...machine.ssh } : { host: '', port: 22, username: '', password: '', private_key: '', passphrase: '' }
                   };
                   this.sshAuthMethod = machine.ssh && machine.ssh.private_key ? 'key' : 'password';
                   this.showMachineForm = true;
               },

               async saveMachine() {
                   try {
                       const url = this.editingMachine ? 
                           '/api/machines/' + this.editingMachine.id : 
                           '/api/machines';
                       
                       const method = this.editingMachine ? 'PUT' : 'POST';
                       
                       const response = await fetch(url, {
                           method: method,
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify(this.machineForm)
                       });

                       if (response.ok) {
                           alert('Servidor salvo com sucesso!');
                           this.showMachineForm = false;
                           await this.loadMachines();
                       } else {
                           alert('Falha ao salvar servidor!');
                       }
                   } catch (error) {
                       console.error('Failed to save machine:', error);
                       alert('Falha ao salvar servidor!');
                   }
               },

               async deleteMachine(machineId) {
                   if (machineId === 'local') {
                       alert('Não é possível excluir o servidor local!');
                       return;
                   }
                   
                   if (!confirm('Tem certeza que deseja excluir este servidor?')) return;
                   
                   try {
                       const response = await fetch('/api/machines/' + machineId, {
                           method: 'DELETE'
                       });
                       
                       if (response.ok) {
                           alert('Servidor excluído com sucesso!');
                           await this.loadMachines();
                           await this.loadSchedules();
                       } else {
                           alert('Falha ao excluir servidor!');
                       }
                   } catch (error) {
                       console.error('Failed to delete machine:', error);
                       alert('Falha ao excluir servidor!');
                   }
               },

               async testMachineConnection(machineId) {
                   try {
                       const response = await fetch('/api/machines/' + machineId + '/test', {
                           method: 'POST'
                       });
                       
                       if (response.ok) {
                           alert('Conexão bem-sucedida!');
                       } else {
                           alert('Falha na conexão!');
                       }
                   } catch (error) {
                       console.error('Connection test failed:', error);
                       alert('Falha na conexão!');
                   }
               },

               async testMachineConfig() {
                   this.testingConnection = true;
                   try {
                       // Clear SSH fields based on auth method
                       if (this.sshAuthMethod === 'key') {
                           this.machineForm.ssh.password = '';
                       } else {
                           this.machineForm.ssh.private_key = '';
                           this.machineForm.ssh.passphrase = '';
                       }

                       const response = await fetch('/api/machines/test-config', {
                           method: 'POST',
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify(this.machineForm)
                       });

                       if (response.ok) {
                           const result = await response.json();
                           alert('✅ ' + result.message);
                       } else {
                           const error = await response.text();
                           alert('❌ Falha na conexão: ' + error);
                       }
                   } catch (error) {
                       console.error('Connection test failed:', error);
                       alert('❌ Erro ao testar conexão: ' + error.message);
                   } finally {
                       this.testingConnection = false;
                   }
               },

               // Schedule management
               resetScheduleForm() {
                   this.scheduleForm = {
                       name: '',
                       description: '',
                       enabled: true,
                       machine_id: '',
                       databases: [],
                       daysOfWeek: [],
                       times: ['09:00']
                   };
                   this.scheduleDatabases = [];
               },

               editSchedule(schedule) {
                   this.editingSchedule = schedule;
                   this.scheduleForm = {
                       name: schedule.name,
                       description: schedule.description || '',
                       enabled: schedule.enabled,
                       machine_id: schedule.machine_id,
                       databases: [...schedule.databases],
                       daysOfWeek: [...schedule.days_of_week],
                       times: [...schedule.times]
                   };
                   this.loadDatabasesForSchedule();
                   this.showScheduleForm = true;
               },

               async saveSchedule() {
                   try {
                       const url = this.editingSchedule ? 
                           '/api/schedules/' + this.editingSchedule.id : 
                           '/api/schedules';
                       
                       const method = this.editingSchedule ? 'PUT' : 'POST';
                       
                       const response = await fetch(url, {
                           method: method,
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify({
                               name: this.scheduleForm.name,
                               description: this.scheduleForm.description,
                               enabled: this.scheduleForm.enabled,
                               machine_id: this.scheduleForm.machine_id,
                               databases: this.scheduleForm.databases,
                               days_of_week: this.scheduleForm.daysOfWeek.map(Number),
                               times: this.scheduleForm.times
                           })
                       });

                       if (response.ok) {
                           alert('Agendamento salvo com sucesso!');
                           this.showScheduleForm = false;
                           await this.loadSchedules();
                           if (this.status.scheduler) {
                               await this.toggleScheduler(); // Restart scheduler
                               await this.toggleScheduler();
                           }
                       } else {
                           alert('Falha ao salvar agendamento!');
                       }
                   } catch (error) {
                       console.error('Failed to save schedule:', error);
                       alert('Falha ao salvar agendamento!');
                   }
               },

               async deleteSchedule(scheduleId) {
                   if (!confirm('Tem certeza que deseja excluir este agendamento?')) return;
                   
                   try {
                       const response = await fetch('/api/schedules/' + scheduleId, {
                           method: 'DELETE'
                       });
                       
                       if (response.ok) {
                           alert('Agendamento excluído com sucesso!');
                           await this.loadSchedules();
                           if (this.status.scheduler) {
                               await this.toggleScheduler(); // Restart scheduler
                               await this.toggleScheduler();
                           }
                       } else {
                           alert('Falha ao excluir agendamento!');
                       }
                   } catch (error) {
                       console.error('Failed to delete schedule:', error);
                       alert('Falha ao excluir agendamento!');
                   }
               },

               async toggleScheduleEnabled(schedule) {
                   try {
                       const response = await fetch('/api/schedules/' + schedule.id, {
                           method: 'PUT',
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify({
                               ...schedule,
                               enabled: !schedule.enabled
                           })
                       });
                       
                       if (response.ok) {
                           await this.loadSchedules();
                           if (this.status.scheduler) {
                               await this.toggleScheduler(); // Restart scheduler
                               await this.toggleScheduler();
                           }
                       }
                   } catch (error) {
                       console.error('Failed to toggle schedule:', error);
                   }
               },

               addTime() {
                   this.scheduleForm.times.push('09:00');
               },

               removeTime(index) {
                   this.scheduleForm.times.splice(index, 1);
               },

               formatDaysOfWeek(days) {
                   return days.map(day => this.daysOfWeek[day]).join(', ');
               },

               async toggleScheduler() {
                   const endpoint = this.status.scheduler ? '/api/scheduler/stop' : '/api/scheduler/start';
                   try {
                       const response = await fetch(endpoint, { method: 'POST' });
                       if (response.ok) {
                           this.status.scheduler = !this.status.scheduler;
                       } else {
                           alert('Falha ao alterar status do scheduler!');
                       }
                   } catch (error) {
                       console.error('Failed to toggle scheduler:', error);
                       alert('Falha ao alterar status do scheduler!');
                   }
               },

               async authenticateGoogle() {
                   try {
                       await this.saveConfig();
                       
                       const response = await fetch('/api/auth/google/url');
                       if (response.ok) {
                           const data = await response.json();
                           const popup = window.open(data.url, 'google-auth', 'width=500,height=600');
                           
                           const pollAuth = setInterval(async () => {
                               try {
                                   if (popup.closed) {
                                       clearInterval(pollAuth);
                                       await this.checkStatus();
                                       if (this.status.google) {
                                           alert('Autenticação Google bem-sucedida!');
                                       }
                                       return;
                                   }
                                   
                                   await this.checkStatus();
                                   if (this.status.google) {
                                       clearInterval(pollAuth);
                                       popup.close();
                                       alert('Autenticação Google bem-sucedida!');
                                   }
                               } catch (error) {
                                   console.error('Error checking auth status:', error);
                               }
                           }, 2000);
                       }
                   } catch (error) {
                       console.error('Google auth failed:', error);
                       alert('Falha na autenticação Google!');
                   }
               },

               async saveConfig() {
                   try {
                       const response = await fetch('/api/config', {
                           method: 'POST',
                           headers: { 'Content-Type': 'application/json' },
                           body: JSON.stringify(this.config)
                       });
                       
                       if (response.ok) {
                           alert('Configurações salvas com sucesso!');
                       } else {
                           alert('Falha ao salvar configurações!');
                       }
                   } catch (error) {
                       console.error('Failed to save config:', error);
                       alert('Falha ao salvar configurações!');
                   }
               },

               formatDate(timestamp) {
                   return new Date(timestamp).toLocaleString('pt-BR');
               },

               formatFileSize(bytes) {
                   if (!bytes) return '-';
                   const sizes = ['Bytes', 'KB', 'MB', 'GB'];
                   const i = Math.floor(Math.log(bytes) / Math.log(1024));
                   return Math.round(bytes / Math.pow(1024, i) * 100) / 100 + ' ' + sizes[i];
               }
           }
       }
   </script>
</body>
</html>`

	t, _ := template.New("index").Parse(tmpl)
	t.Execute(w, nil)
}

// Config handlers
func (h *Handler) GetConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config)
}

func (h *Handler) UpdateConfigHandler(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update Google configuration
	if google, ok := updates["google"].(map[string]interface{}); ok {
		if clientID, ok := google["client_id"].(string); ok {
			h.config.Google.ClientID = clientID
		}
		if clientSecret, ok := google["client_secret"].(string); ok {
			h.config.Google.ClientSecret = clientSecret
		}
		if sheetID, ok := google["sheet_id"].(string); ok {
			h.config.Google.SheetID = sheetID
		}
		if driveFolder, ok := google["drive_folder"].(string); ok {
			h.config.Google.DriveFolder = driveFolder
		}
	}

	if err := h.config.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TestMySQLHandler(w http.ResponseWriter, r *http.Request) {
	var mysqlConfig config.MySQLConfig
	if err := json.NewDecoder(r.Body).Decode(&mysqlConfig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Test connection using the local machine for backward compatibility
	localMachine, err := h.config.GetMachine("local")
	if err != nil {
		http.Error(w, "Local machine not found", http.StatusInternalServerError)
		return
	}

	originalConfig := localMachine.MySQL
	localMachine.MySQL = mysqlConfig

	err = h.backupService.TestMachineConnection("local")

	if err != nil {
		localMachine.MySQL = originalConfig
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update the local machine config
	if err := h.config.UpdateMachine("local", *localMachine); err != nil {
		localMachine.MySQL = originalConfig
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Database handlers
func (h *Handler) GetDatabasesHandler(w http.ResponseWriter, r *http.Request) {
	databases, err := h.backupService.GetDatabases()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(databases)
}

// Backup handlers
func (h *Handler) CreateManualBackupHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Databases []string `json:"databases"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Minute)
	defer cancel()

	results, err := h.backupService.CreateBackup(ctx, req.Databases)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) CreateMachineBackupHandler(w http.ResponseWriter, r *http.Request) {
	machineID := strings.TrimPrefix(r.URL.Path, "/api/machines/")
	machineID = strings.TrimSuffix(machineID, "/backup")

	var req struct {
		Databases []string `json:"databases"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Minute)
	defer cancel()

	results, err := h.backupService.CreateMachineBackup(ctx, machineID, req.Databases)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) GetBackupLogsHandler(w http.ResponseWriter, r *http.Request) {
	logs, err := h.config.GetBackupLogs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// Schedule handlers
func (h *Handler) GetSchedulesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config.Scheduler.Schedules)
}

func (h *Handler) CreateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	var schedule config.Schedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.config.AddSchedule(schedule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdateScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := strings.TrimPrefix(r.URL.Path, "/api/schedules/")

	var schedule config.Schedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.config.UpdateSchedule(scheduleID, schedule); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteScheduleHandler(w http.ResponseWriter, r *http.Request) {
	scheduleID := strings.TrimPrefix(r.URL.Path, "/api/schedules/")

	if err := h.config.DeleteSchedule(scheduleID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Scheduler control handlers
func (h *Handler) GetSchedulerStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"running":   h.schedulerService.IsRunning(),
		"schedules": len(h.config.GetEnabledSchedules()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) StartSchedulerHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.schedulerService.Start(context.Background()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) StopSchedulerHandler(w http.ResponseWriter, r *http.Request) {
	h.schedulerService.Stop()
	w.WriteHeader(http.StatusOK)
}

// Google Auth handlers
func (h *Handler) GetGoogleAuthURLHandler(w http.ResponseWriter, r *http.Request) {
	if !h.config.IsGoogleConfigured() {
		http.Error(w, "Google not configured", http.StatusBadRequest)
		return
	}

	googleClient := google.NewClient(h.config)
	authURL := googleClient.GetAuthURL()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

func (h *Handler) GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No authorization code", http.StatusBadRequest)
		return
	}

	googleClient := google.NewClient(h.config)
	if err := googleClient.ExchangeCode(code); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<html><body><script>window.close();</script><p>Autenticação concluída!</p></body></html>`))
}

func (h *Handler) GetGoogleAuthStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]bool{
		"authenticated": h.config.IsGoogleAuthenticated(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Machine management handlers
func (h *Handler) GetMachinesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config.Machines)
}

func (h *Handler) CreateMachineHandler(w http.ResponseWriter, r *http.Request) {
	var machine config.Machine
	if err := json.NewDecoder(r.Body).Decode(&machine); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.config.AddMachine(machine); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdateMachineHandler(w http.ResponseWriter, r *http.Request) {
	machineID := strings.TrimPrefix(r.URL.Path, "/api/machines/")

	var machine config.Machine
	if err := json.NewDecoder(r.Body).Decode(&machine); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.config.UpdateMachine(machineID, machine); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeleteMachineHandler(w http.ResponseWriter, r *http.Request) {
	machineID := strings.TrimPrefix(r.URL.Path, "/api/machines/")

	if err := h.config.DeleteMachine(machineID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) TestMachineConnectionHandler(w http.ResponseWriter, r *http.Request) {
	machineID := strings.TrimPrefix(r.URL.Path, "/api/machines/")
	machineID = strings.TrimSuffix(machineID, "/test")

	if err := h.backupService.TestMachineConnection(machineID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetMachineDatabasesHandler(w http.ResponseWriter, r *http.Request) {
	machineID := strings.TrimPrefix(r.URL.Path, "/api/machines/")
	machineID = strings.TrimSuffix(machineID, "/databases")

	databases, err := h.backupService.GetMachineDatabases(machineID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(databases)
}

func (h *Handler) TestMachineConfigHandler(w http.ResponseWriter, r *http.Request) {
	var machine config.Machine
	if err := json.NewDecoder(r.Body).Decode(&machine); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a temporary machine for testing
	tempMachine := machine
	tempMachine.ID = "temp_test"

	// Test the connection
	var err error
	if tempMachine.Type == "remote" {
		// Test SSH connection first
		sshClient := ssh.NewClient(&tempMachine.SSH)
		if err = sshClient.TestConnection(); err != nil {
			http.Error(w, fmt.Sprintf("SSH connection failed: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Test MySQL connection
	backupService := backup.NewService(h.config)

	// Temporarily add machine to config for testing
	originalMachines := h.config.Machines
	h.config.Machines = append(h.config.Machines, tempMachine)

	err = backupService.TestMachineConnection("temp_test")

	// Restore original machines
	h.config.Machines = originalMachines

	if err != nil {
		http.Error(w, fmt.Sprintf("MySQL connection failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Connection successful"})
}
