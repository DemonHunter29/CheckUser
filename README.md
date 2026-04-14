# CheckUser

Serviço leve de validação de usuário + limite de conexões por dispositivo para
usuários de sistema (SSH/OpenVPN). Lê `/etc/passwd`, consulta `chage` para
verificar expiração e mantém um banco SQLite com os `deviceId` já registrados
por usuário.

## API

| Método | Rota                            | Descrição                                           |
|--------|---------------------------------|-----------------------------------------------------|
| GET    | `/check/:username?deviceId=X`   | Valida usuário + registra device, devolve contadores |
| GET    | `/details/:username`            | Detalhes do usuário                                  |
| GET    | `/count`                        | Total de conexões ativas                             |
| GET    | `/devices/list`                 | Lista todos os devices                               |
| GET    | `/devices/list/:username`       | Lista devices de um usuário                          |
| GET    | `/devices/count`                | Total de devices                                     |

Resposta de `/check/`:

```json
{
  "id": 1,
  "username": "user1",
  "expiration_date": "31/12/2026",
  "expiration_days": 200,
  "limit_connections": 2,
  "count_connections": 1
}
```

## Build

```bash
go build -ldflags="-w -s -buildid=" -trimpath -o checkuser ./src
```

## Rodar (foreground)

```bash
./checkuser --start --port 5000
# com SSL (cert.pem/key.pem no dir atual):
./checkuser --start --port 5000 --ssl
```

## Instalar como serviço

```bash
sudo bash install.sh
```

Menu interativo com opções de instalar/remover/status. O binário é copiado para
`/usr/local/bin/checkuser` e o serviço fica em `/etc/systemd/system/checkuser.service`.

## Integração com o painel

No painel, edite uma configuração e preencha o campo **URL Check User** com
a base da URL do serviço (sem `/check/` no final):

```
http://seu-servidor.com:5000
```

O app anexa `/check/<username>?deviceId=<uuid>` antes de subir o túnel e bloqueia
a conexão se o usuário estiver expirado ou acima do limite.

## Licença

MIT — veja [LICENSE](LICENSE).
