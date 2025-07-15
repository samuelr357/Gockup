FROM golang:1.21-alpine AS builder

# Instala o mariadb-client (no caso de MariaDB, usa-se mariadb-dump)
RUN apk add --no-cache mariadb-client

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . . 
RUN go build -o mysql-backup-system main.go

FROM alpine:latest

# Instalar mariadb-client (mariadb-dump), openssh e ca-certificates
RUN apk add --no-cache mariadb-client ca-certificates openssh

# Criar pastas
RUN mkdir -p /app/backups /app/logs /var/run/sshd /root/.ssh

WORKDIR /app

# Copiar binário do builder
COPY --from=builder /app/mysql-backup-system . 

# Configuração do SSH
RUN echo "root:root" | chpasswd && \
    ssh-keygen -A && \
    sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#Port 22/Port 22/' /etc/ssh/sshd_config && \
    echo "PasswordAuthentication yes" >> /etc/ssh/sshd_config

# Expõe as portas
EXPOSE 8030 22

# Comando para rodar SSH + aplicação de backup
CMD /usr/sbin/sshd && ./mysql-backup-system
