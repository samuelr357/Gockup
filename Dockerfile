FROM golang:1.24-alpine AS builder

# Instalar o mariadb-client (mariadb-dump) e tzdata para configuração de fuso horário
RUN apk add --no-cache mariadb-client tzdata

# Configurar o fuso horário para o Brasil (GMT-3)
RUN cp /usr/share/zoneinfo/America/Sao_Paulo /etc/localtime && \
    echo "America/Sao_Paulo" > /etc/timezone

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . . 
RUN go build -o mysql-backup-system main.go

FROM alpine:latest

# Instalar o mariadb-client (mariadb-dump), openssh, ca-certificates e tzdata
RUN apk add --no-cache mariadb-client ca-certificates openssh tzdata mysql-client

# Configurar o fuso horário para o Brasil (GMT-3)
RUN cp /usr/share/zoneinfo/America/Sao_Paulo /etc/localtime && \
    echo "America/Sao_Paulo" > /etc/timezone

# Criar pastas necessárias
RUN mkdir -p /app/backups /app/logs /var/run/sshd /root/.ssh

WORKDIR /app

# Copiar o binário gerado na etapa de builder
COPY --from=builder /app/mysql-backup-system . 

# Configuração do SSH
RUN echo "root:root" | chpasswd && \
    ssh-keygen -A && \
    sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#Port 22/Port 22/' /etc/ssh/sshd_config && \
    echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config

# Expor as portas
EXPOSE 8030 22

# Comando para rodar SSH + o serviço de backup
CMD /usr/sbin/sshd && ./mysql-backup-system
