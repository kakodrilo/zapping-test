# Contexto del Proyecto: Zapping Live Streaming

## Objetivo general

Construir una aplicación de livestreaming HLS en Go con:
- Autenticación JWT (stateless, sin sesiones en BD)
- Un stream global compartido: todos los clientes ven el mismo segmento al mismo tiempo
- Rotación automática de segmentos cada 10 segundos (simulación de livestream real)
- Frontend en HTML + JS + HLS.js servido como estático desde el backend Go
- Backend y base de datos orquestados en Docker

---

## Requisito de origen (PDF)

- 3 páginas web: Crear Cuenta, Login, Player
- DB para registro de usuarios
- Solo usuarios registrados acceden al Player
- Backend Go que entregue un HLS Livestream
- Segmentos de 10 segundos cada uno
- Cada request devuelve 3 segmentos (30s de video)
- Cada 10 segundos: se elimina el primer segmento de la lista y se agrega uno nuevo al final
- `EXT-X-MEDIA-SEQUENCE` incrementa con cada rotación
- Frontend con HLS.js o Video.js, puede usar Bootstrap y jQuery

---

## Arquitectura decidida

```
                    ┌─────────────────────────────────────────┐
                    │           Docker Network interna         │
  Browser  ──────►  │  Go App :8080  ──────►  NGINX :80       │
  (cliente)         │  (auth + proxy)         (media server)   │
                    └─────────────────────────────────────────┘
                              │
                           MySQL :3306
```

### Por qué este diseño
A gran escala, los segmentos `.ts` viven en un servidor de medios dedicado (o CDN), nunca en el mismo proceso que maneja auth y lógica de negocio. El contenedor NGINX emula ese media server. Go actúa como gateway autenticado: valida el JWT y hace proxy al segmento real. NGINX **no está expuesto** al exterior.

### Contenedor NGINX (media server)
- Imagen custom basada en `nginx:alpine` con su propio `nginx/Dockerfile`
- Los segmentos `.ts` se copian **dentro de la imagen** durante el build (`COPY`)
- Solo accesible dentro de la red Docker interna (`livestream_network`) — sin puerto expuesto al host
- Sirve los archivos `.ts` de forma estática desde `/usr/share/nginx/html/segments/`
- `docker-compose up --build` es suficiente para reproducir todo; sin dependencias del host

### Backend Go
- Puerto `8080` expuesto al host
- Sirve: API REST + Frontend HTML estático + playlist HLS + **proxy de segmentos**
- Los segmentos `.ts` **no se sirven directamente** desde Go; Go los solicita a NGINX internamente y los reenvía al cliente tras validar el JWT
- Variable de entorno `MEDIA_SERVER_URL=http://nginx/segments` para apuntar al NGINX interno

### Estado global de la playlist (en memoria)
- Una estructura en memoria mantiene la ventana actual de **3 segmentos**
- Un goroutine con `time.Ticker` de 10 segundos rota la ventana: elimina el primero, agrega el siguiente
- `EXT-X-MEDIA-SEQUENCE` incrementa globalmente con cada rotación
- Todos los clientes que soliciten `/stream/playlist.m3u8` reciben la misma ventana en ese instante
- Esto simula un "livestream real" donde 2 navegadores distintos ven el mismo contenido simultáneamente

### Por qué múltiples clientes ven lo mismo
No son streams independientes. Hay un único reloj global. Si el cliente A y el cliente B piden la playlist al mismo tiempo, ambos reciben los mismos 3 segmentos. Es el mismo modelo que un canal de TV en vivo.

### Frontend
- HTML + JavaScript vanilla + Bootstrap 5 + HLS.js
- Servido como archivos estáticos desde `backend/static/`
- 3 páginas: `register.html`, `login.html`, `player.html`
- El JWT se guarda en `localStorage` y se envía en el header `Authorization: Bearer <token>` en cada request

### Autenticación JWT
- Al hacer login, el backend genera un JWT firmado con expiración de **1 hora**
- El frontend ejecuta un `setInterval` cada **50 minutos** que llama a `GET /api/refresh` y reemplaza el token en `localStorage`
- Sin sesiones en BD
- Los endpoints `/stream/playlist.m3u8` y `/stream/segments/:file` requieren JWT válido
- Si el token expira mientras el usuario está en el Player (ej: cerró el laptop más de 1 hora), el frontend detecta el 401 y redirige a login

**Por qué 1 hora + refresh silencioso:**
- 5 minutos era incompatible con HLS.js (no puede interceptar headers de respuesta para renovar el token)
- El refresh silencioso cada 50 min desacopla la renovación de los requests de HLS.js
- 1 hora es el estándar de industria para access tokens en aplicaciones de consumo

---

## Estructura del proyecto

```
zapping-test/
├── backend/
│   ├── go.mod / go.sum
│   ├── main.go                    ← pendiente
│   ├── internal/
│   │   ├── db/mysql.go            ← conexión MySQL (listo)
│   │   ├── auth/jwt.go            ← pendiente
│   │   ├── handlers/              ← pendiente
│   │   │   ├── auth.go
│   │   │   └── stream.go          ← incluye proxy a NGINX
│   │   └── stream/rotation.go     ← lógica global de rotación (pendiente)
│   └── static/
│       ├── register.html          ← pendiente
│       ├── login.html             ← pendiente
│       └── player.html            ← pendiente
├── segments/                      ← 63 archivos .ts (NO en git; solo pre-build)
├── nginx/
│   ├── Dockerfile                 ← pendiente (copia segments/ al build)
│   └── nginx.conf                 ← pendiente (config del media server)
├── database/
│   ├── init_db/
│   │   ├── schema.sql             ← listo (tablas users y streams_metadata)
│   │   ├── 01_sp_register_user.sql
│   │   └── 02_sp_get_user.sql
│   └── mysql_data/                ← volumen persistente MySQL
├── Dockerfile                     ← necesita corrección de versión Go
├── docker-compose.yml             ← necesita agregar servicio nginx
├── .env                           ← necesita agregar MEDIA_SERVER_URL
└── .env.example                   ← listo
```

### Por qué `segments/` está separado de `backend/static/`
El Go Dockerfile copia `backend/static/` (solo HTML/JS/CSS). El NGINX Dockerfile copia `segments/` (solo `.ts`). Así ninguno de los dos carga archivos que no necesita, y el binario Go no queda inflado con 63 archivos de video.

---

## Estado actual

### Listo
- `docker-compose up` funciona correctamente
- Go app se conecta a MySQL correctamente
- phpMyAdmin accesible en `http://localhost:8081`
- Tablas `users` y `streams_metadata` creadas vía schema.sql
- Stored procedures: `sp_register_user`, `sp_get_user_by_email`
- `backend/internal/db/mysql.go` funcional
- Variables de entorno en `.env`

### Pendiente

**Setup único (pre-build, no repetible via Docker):**
- [ ] Copiar los 63 segmentos `.ts` a `segments/` en la raíz del proyecto
- [ ] Agregar `segments/` al `.gitignore`

**Infraestructura Docker:**
- [x] Corregir versión de Go en `Dockerfile` (→ `1.23`)
- [ ] Crear `nginx/Dockerfile` (FROM nginx:alpine + COPY segments/ + mime types)
- [ ] Crear `nginx/nginx.conf` (mime type `video/mp2t` para `.ts`)
- [ ] Agregar servicio `nginx` al `docker-compose.yml` (build: ./nginx, sin puerto al host)
- [ ] Agregar `MEDIA_SERVER_URL=http://nginx/segments` al `.env`

**Base de datos:**
- [ ] Actualizar `schema.sql`: agregar `description` e `initial_offset` a `streams_metadata`
- [ ] Agregar seed de al menos 2 streams activos en `init_db`

**Backend Go:**
- [ ] Crear `backend/main.go` con router HTTP
- [ ] Crear `backend/internal/auth/jwt.go` (1h, refresh endpoint)
- [ ] Crear `backend/internal/stream/rotation.go` (goroutine + RWMutex por stream)
- [ ] Crear `backend/internal/handlers/auth.go` (register, login, refresh)
- [ ] Crear `backend/internal/handlers/stream.go` (lista streams, playlist, proxy NGINX)

**Frontend:**
- [ ] Crear `backend/static/register.html`
- [ ] Crear `backend/static/login.html`
- [ ] Crear `backend/static/player.html` con HLS.js + xhrSetup para JWT + setInterval refresh

---

## Endpoints API

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| POST | `/api/register` | No | Registro de usuario |
| POST | `/api/login` | No | Login, devuelve JWT (1h) |
| GET | `/api/refresh` | JWT | Renueva el token (llamado cada 50 min por el frontend) |
| GET | `/api/streams` | JWT | Lista de streams activos desde DB |
| GET | `/stream/{id}/playlist.m3u8` | JWT | Playlist HLS del stream `id` |
| GET | `/stream/{id}/segments/:file` | JWT | Proxy del `.ts` via NGINX interno |
| GET | `/` | No | Redirige a `login.html` |

---

## Lógica de rotación (algoritmo)

### Stream único global (por stream_id)
```
Estado inicial al arrancar el servidor (por cada stream activo en DB):
  - segments   = [seg_001.ts, seg_002.ts, ..., seg_063.ts]  ← leído del FS al inicio
  - windowStart = initialOffset  ← definido en streams_metadata.initial_offset
  - mediaSequence = 0
  - mu sync.RWMutex  ← protege windowStart y mediaSequence

Cada 10 segundos (goroutine con time.Ticker, uno por stream):
  - mu.Lock()
  - windowStart = (windowStart + 1) % len(segments)   ← loop circular infinito
  - mediaSequence++
  - mu.Unlock()

Al responder GET /stream/{stream_id}/playlist.m3u8:
  - mu.RLock()
  - window = segments[windowStart], segments[(windowStart+1)%N], segments[(windowStart+2)%N]
  - seq    = mediaSequence
  - mu.RUnlock()
  - Retornar m3u8 con los 3 segmentos y EXT-X-MEDIA-SEQUENCE = seq
```

### Loop infinito — el stream nunca termina
- `windowStart` avanza con módulo: cuando llega al último segmento, vuelve al primero automáticamente
- `EXT-X-MEDIA-SEQUENCE` sigue incrementando para siempre (nunca resetea)
- No hay pantalla de "fin de stream" — es un canal 24/7 como TV en vivo
- Ciclo completo con 63 segmentos: **63 × 10s = 630 segundos ≈ 10.5 minutos**, luego repite

### Múltiples streams en paralelo
- Cada stream definido en `streams_metadata` tiene su propio goroutine y su propio estado en memoria
- Streams desfasados mediante `initial_offset` en DB: stream 2 empieza en segmento 21 (⅓ del ciclo después)
- Resultado: dos streams muestran contenido diferente al mismo tiempo, como canales de TV distintos

---

## Base de datos

### Tablas

**`users`**
| Campo | Tipo | Notas |
|-------|------|-------|
| id | INT AUTO_INCREMENT PK | |
| name | VARCHAR(100) | |
| email | VARCHAR(150) UNIQUE | |
| password_hash | VARCHAR(255) | bcrypt desde Go |
| created_at | TIMESTAMP | DEFAULT NOW() |

**`streams_metadata`**
| Campo | Tipo | Notas |
|-------|------|-------|
| id | INT AUTO_INCREMENT PK | stream_id usado en rutas API |
| title | VARCHAR(100) | Nombre visible en el Player |
| description | VARCHAR(255) | Descripción opcional |
| initial_offset | INT DEFAULT 0 | Posición inicial en el pool de segmentos |
| is_active | BOOLEAN DEFAULT TRUE | Si está disponible para reproducir |

**Por qué `streams_metadata` en BD:**
Agregar un nuevo canal no requiere tocar código. Solo se inserta una fila con `initial_offset` distinto y el servidor la levanta como un goroutine independiente. El Player lista los streams activos consultando la BD al inicio.

> **Nota:** El schema actual ya tiene `streams_metadata` pero le falta `description` e `initial_offset`. Hay que actualizarlo.

### Stored procedures
- `sp_register_user(name, email, password_hash)` — listo
- `sp_get_user_by_email(email)` — listo (`02_sp_get_user_by_email.sql`)

---

## Variables de entorno (.env)

```
DB_HOST=db
DB_PORT=3306
MYSQL_DATABASE=livestream_app
MYSQL_USER=...
MYSQL_PASSWORD=...
MYSQL_ROOT_PASSWORD=...
APP_PORT=8080
PMA_PORT=8081
JWT_SECRET=...
```

---

## Decisiones de diseño importantes

| Decisión | Elección | Razón |
|----------|----------|-------|
| Frontend | HTML + JS + HLS.js | Lo que pide el PDF; sin build step, Go lo sirve directamente |
| Auth | JWT 1 hora + refresh silencioso cada 50 min | Compatible con HLS.js; seguro sin depender de headers de respuesta |
| Rotación | Goroutine + RWMutex por stream, loop circular infinito | O(1) por request, thread-safe, el stream nunca termina |
| Segmentos | Contenedor NGINX interno (no expuesto) + Go proxy | Emula arquitectura real con media server separado; Go autentica y reenvía |
| Versión Go | 1.23 | La más reciente estable; `1.25.0` del Dockerfile original no existe |
| Streams | Un pool global compartido | Todos los clientes ven el mismo instante del stream |

---

## Flujo de reproducción

```
1. Usuario coloca los 63 .ts en segments/ (una vez)
2. docker-compose up --build
   ├── Go image: compila binario + copia backend/static/ (HTML/JS)
   ├── NGINX image: nginx:alpine + COPY segments/ → /usr/share/nginx/html/segments/
   └── MySQL image: oficial + scripts init_db/
3. Resultado: todo corre desde imágenes, sin dependencias del host
```

---

## Notas técnicas

- Los segmentos `.ts` NO se deben incluir en git — agregar `segments/` al `.gitignore`
- No exponer credenciales en código; todo via variables de entorno
- Go Dockerfile copia solo `backend/static/` (HTML/JS/CSS) — sin los `.ts`
- NGINX Dockerfile copia solo `segments/` — sin código Go
- `EXT-X-TARGETDURATION` en el m3u8 debe ser `10` (duración real de cada segmento)
- NGINX solo está en la red Docker interna; ningún cliente puede pedir un `.ts` directamente sin pasar por Go
- El proxy en Go debe usar `io.Copy` para reenviar el body del segmento — nunca cargar en memoria completo (eficiencia de RAM)
- Si NGINX no responde, Go devuelve 502 al cliente
- El único volumen persistente necesario es MySQL data (`database/mysql_data`)
