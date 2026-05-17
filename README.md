# Zapping Livestream — Prueba Técnica

Plataforma de livestreaming HLS simulado con autenticación JWT, construida en Go. Permite registrar usuarios, iniciar sesión y ver canales de televisión en vivo desde un player web, todo corriendo en Docker.

---

## Índice

1. [Qué hace la solución](#1-qué-hace-la-solución)
2. [Arquitectura general](#2-arquitectura-general)
3. [Servicios Docker](#3-servicios-docker)
4. [Cómo funciona el livestreaming HLS](#4-cómo-funciona-el-livestreaming-hls)
5. [Autenticación JWT](#5-autenticación-jwt)
6. [Decisiones adicionales no requeridas](#6-decisiones-adicionales-no-requeridas)
7. [Gestión de memoria RAM](#7-gestión-de-memoria-ram)
8. [Setup desde cero](#8-setup-desde-cero)
9. [Agregar un nuevo stream](#9-agregar-un-nuevo-stream)

---

## 1. Qué hace la solución

El sistema expone tres páginas web:

| Página | Ruta | Acceso |
|---|---|---|
| Registro | `/register.html` | Público |
| Login | `/login.html` | Público |
| Player | `/player.html` | Solo usuarios autenticados |

El player muestra una lista de canales disponibles. Al seleccionar uno, reproduce un livestream HLS usando **HLS.js**. El stream se simula haciendo rotar una ventana de 3 segmentos de video pre-grabados cada 10 segundos, tal como indica el protocolo HLS live.

---

## 2. Arquitectura general

El backend Go sigue **Clean Architecture (Hexagonal)** con cuatro capas explícitas:

```
┌──────────────────────────────────────────────┐
│  main.go — Composición (wiring)              │
├──────────────────────────────────────────────┤
│  adapter/  — HTTP handlers, repos, JWT, media│  ← capa exterior
├──────────────────────────────────────────────┤
│  usecase/  — Lógica de negocio               │  ← application
├──────────────────────────────────────────────┤
│  port/     — Interfaces (boundaries)         │
├──────────────────────────────────────────────┤
│  domain/   — Entidades y errores de dominio  │  ← núcleo
└──────────────────────────────────────────────┘
```

**Regla de dependencias:** las capas externas conocen a las internas, nunca al revés. `domain` no importa ningún paquete interno. Los use cases dependen solo de interfaces (`port`), nunca de implementaciones concretas.

### Estructura de directorios

```
zapping-test/
├── backend/
│   ├── main.go
│   └── internal/
│       ├── domain/          # Stream, Segment, User, errores de negocio
│       ├── port/            # Interfaces: repositorios, servicios, media client
│       ├── usecase/
│       │   ├── authuc/      # Register, Login, Refresh
│       │   └── streamuc/    # Playlist HLS, proxy de segmentos
│       └── adapter/
│           ├── handler/     # HTTP handlers + middlewares (CORS, Auth)
│           ├── repo/        # MySQL: usuarios, streams, segmentos
│           ├── jwtadapter/  # Firma y validación JWT (HS256)
│           └── media/       # HTTP client hacia el media server interno
├── frontend/                # HTML + HLS.js + CSS
├── nginx/                   # Media server interno (sirve los .ts)
├── gateway/                 # Reverse proxy HTTPS (entrada pública)
├── database/
│   └── init_db/             # SQL ejecutado automáticamente por MySQL al iniciar
├── segments/                # Archivos .ts pre-grabados
└── certs/                   # Certificados TLS autofirmados
```

---

## 3. Servicios Docker

El `docker-compose.yml` levanta **6 servicios**:

```
Internet
    │  :3000 (HTTPS)   :8080 (HTTPS)   :8081 (HTTPS)
    ▼
┌─────────────────────────────────────────────────────┐
│  gateway  (nginx — reverse proxy SSL)               │
└──────┬──────────────────┬──────────────────┬────────┘
       │                  │                  │
       ▼                  ▼                  ▼
  ┌─────────┐       ┌──────────┐      ┌──────────────┐
  │frontend │       │   app    │      │  phpmyadmin  │
  │(node/   │       │  (Go)    │      │              │
  │ serve)  │       │  :8080   │      │              │
  └─────────┘       └────┬─────┘      └──────────────┘
                         │
              ┌──────────┴──────────┐
              │                     │
              ▼                     ▼
         ┌─────────┐          ┌──────────┐
         │   db    │          │  nginx   │
         │(MySQL)  │          │(media    │
         │  :3306  │          │ server)  │
         └─────────┘          └──────────┘
```

| Servicio | Imagen | Rol | Expuesto |
|---|---|---|---|
| `gateway` | nginx:alpine | Reverse proxy HTTPS, TLS termination | :3000, :8080, :8081 |
| `app` | Go multi-stage | API REST + HLS endpoint | Solo interno |
| `frontend` | node:alpine + serve | Sirve los HTML estáticos | Solo interno |
| `nginx` | nginx:alpine | Media server: sirve los `.ts` | Solo interno |
| `db` | mysql:8.0 | Base de datos de usuarios y streams | Solo interno |
| `phpmyadmin` | phpmyadmin:latest | Administración de DB | Solo interno vía gateway |

**El media server nginx nunca se expone directamente.** Los segmentos `.ts` solo son accesibles a través del servicio `app` (Go), que valida el JWT antes de hacer proxy.

---

## 4. Cómo funciona el livestreaming HLS

### El protocolo HLS live

HLS live funciona con un archivo de playlist `.m3u8` que cambia con el tiempo. El cliente (HLS.js) lo refresca cada cierto intervalo y reproduce los segmentos que aparecen. La etiqueta `EXT-X-MEDIA-SEQUENCE` indica cuántos segmentos han sido "emitidos" desde el inicio — debe aumentar monotónicamente y nunca retroceder.

Ejemplo de playlist devuelta por el servidor:

```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:4821
#EXTINF:10.000000,
/stream/1/segments/segment21.ts
#EXTINF:10.000000,
/stream/1/segments/segment22.ts
#EXTINF:10.000000,
/stream/1/segments/segment23.ts
```

### Posición determinista por reloj (`started_at`)

Cada stream en la base de datos tiene un campo `started_at TIMESTAMP`. La posición actual se calcula en cada request con aritmética pura, sin estado mutable:

```
rotations    = int(time.Since(started_at) / 10s)
windowStart  = rotations % totalSegments
sequence     = rotations          ← crece sin límite, nunca retrocede
```

Los tres segmentos del window se obtienen por sus posiciones con un `IN (?, ?, ?)` a la DB.

**Ventajas de este enfoque:**
- **Sobrevive reinicios**: al reiniciar el servicio, la posición se recalcula igual a partir del mismo timestamp → el stream nunca "retrocede"
- **Múltiples instancias**: N réplicas del servicio devuelven exactamente el mismo m3u8 para el mismo stream al mismo instante, sin coordinación entre ellas
- **Sin estado compartido mutable**: no hay goroutines de rotación, no hay mutex por stream
- **El ciclo es infinito**: el módulo garantiza que al llegar al último segmento se vuelve al primero automáticamente

### Desfase entre canales

Para simular dos transmisiones en vivo independientes sin necesidad de dos videos distintos, se usa el mismo contenido (`big_bug_bunny`) en ambos canales pero con un `started_at` diferente. Esto hace que cada canal esté en un punto distinto del ciclo en todo momento, dando la ilusión de dos señales en vivo separadas.

Los dos streams (`Kids4Fun` y `CineArte`) tienen `started_at` con **5 minutos de diferencia**:

```
Kids4Fun  → started_at: '2020-01-01 00:00:00'
CineArte  → started_at: '2020-01-01 00:05:00'
```

5 minutos = 300 segundos = **30 rotaciones de diferencia** (30 × 10s). En cualquier momento ambos canales muestran segmentos distintos dentro del mismo contenido.

### Proxy de segmentos `.ts`

Cuando HLS.js solicita un segmento, el flujo es:

```
HLS.js → gateway (HTTPS) → app Go (valida JWT) → nginx interno → archivo .ts
```

El servicio Go hace **streaming del segmento byte a byte** hacia el cliente usando `io.Copy` — el archivo nunca se carga completo en memoria. Solo 32 KB circulan en cada instante.

---

## 5. Autenticación JWT

### Flujo

```
POST /api/register   →  bcrypt(password) → DB → 201 Created
POST /api/login      →  bcrypt.Compare   → JWT signed HS256 (1h) → {token}
GET  /api/refresh    →  valida JWT → nuevo JWT con timestamps frescos
```

El token JWT contiene `user_id` y `email` en el payload. El middleware `AuthMiddleware` lo extrae del header `Authorization: Bearer <token>` y lo inyecta en el `context.Context` de cada request protegido.

### Rutas protegidas

```
GET  /api/streams                        ← requiere JWT
GET  /stream/{id}/playlist.m3u8          ← requiere JWT
GET  /stream/{id}/segments/{file}        ← requiere JWT
```

Un usuario no autenticado no puede acceder a ningún segmento de video. El player redirige al login si no hay token en `localStorage`.

### Refresco automático

El frontend refresca el token cada **50 minutos** en background (antes de que expire la hora). Así la sesión se mantiene activa mientras el usuario está viendo contenido.

---

## 6. Decisiones adicionales no requeridas

### Clean Architecture estricta

Las interfaces están en `port/` y en los propios consumidores (handlers locales). Ninguna capa interna conoce los detalles de implementación de las externas. El `main.go` actúa exclusivamente como punto de composición (wiring).

Puntos específicos:
- `jwtadapter.NewService(secret string)` recibe el secret inyectado desde `main`, no lo lee del entorno por su cuenta
- `db.InitDB()` retorna `(*sql.DB, error)` — sin global mutable. `main` gestiona el ciclo de vida de la conexión
- Los handlers definen sus propias interfaces locales (`streamUseCase`, `authUseCase`) en lugar de depender de tipos concretos — Dependency Inversion puro en Go

### Gateway HTTPS con TLS autofirmado

El acceso público pasa siempre por HTTPS. El gateway nginx termina TLS con certificados en `certs/`. El script `certs/generate.sh` los genera con `openssl`.

### Media server interno aislado

El nginx que sirve los `.ts` no tiene ningún puerto expuesto al host. Solo el servicio `app` puede acceder a él por red interna de Docker. Esto garantiza que ningún segmento es accesible sin pasar por la validación JWT.

### phpMyAdmin incluido

Disponible en `https://localhost:8081` para inspeccionar la base de datos sin necesidad de cliente externo.

### Player con experiencia tipo TV

El frontend incluye:
- Cambio de canal con animación de "zapping" (blur + transición)
- Indicador de señal en vivo (punto pulsante)
- Atajos de teclado: `Espacio` play/pause, `M` mute, `F` fullscreen, `C` sidebar, `↑↓` volumen, `←→` cambio de canal, `?` ayuda
- Modo oscuro/claro con persistencia en `localStorage`
- Miniaturas con gradientes procedurales por canal
- Recuperación automática de errores de red (retry a los 2s)

### Límites del pool de MySQL

```go
database.SetMaxOpenConns(10)
database.SetMaxIdleConns(5)
database.SetConnMaxLifetime(5 * time.Minute)
```

Sin estos límites, `database/sql` abre conexiones ilimitadas bajo carga concurrente.

### Timeout explícito en el cliente HTTP hacia nginx

```go
client: &http.Client{Timeout: 30 * time.Second}
```

Sin timeout, una respuesta colgada de nginx bloquea la goroutine del request indefinidamente.

### `config.js` configurable por variable de entorno

La URL de la API se inyecta en `config.js` al arrancar el contenedor mediante un script `entrypoint.sh`, sin necesidad de reconstruir la imagen. Basta con cambiar `API_BASE` en `.env`.

---

## 7. Gestión de memoria RAM

### Estado permanente por stream: ~140 bytes

```
streamMeta {
  segCount       int        →   8 bytes
  targetDuration int        →   8 bytes
  startedAt      time.Time  →  24 bytes
}
sync.Map entry overhead     → ~100 bytes
─────────────────────────────────────────
Total por stream activo     → ~140 bytes
```

No hay goroutines de rotación, no hay mutex por stream, no hay listas de segmentos en memoria.

### Por request de playlist: ~700 bytes (todo GC'd)

El cálculo de posición usa solo aritmética de enteros en el stack (0 heap). Los 3 segmentos se cargan desde DB únicamente para el request actual y el GC los recupera al finalizar.

### Por request de segmento `.ts`: 32 KB pico

`io.Copy` usa un buffer interno de 32 KB que rueda entre nginx y el cliente. Un segmento de 10s puede pesar 1–4 MB dependiendo del bitrate; en ningún momento se carga completo en memoria.

### Pool MySQL: techo de ~4 MB

Con `MaxOpenConns(10)`, el peor caso es 10 conexiones × ~400 KB de buffers por conexión = **4 MB controlados y predecibles**.


---

## 8. Setup desde cero

### Requisitos previos

- Docker Desktop instalado y corriendo
- Los archivos `.ts` de segmentos de video

### Paso 1 — Certificados TLS

Requiere `openssl`. Ejecutar desde la **raíz del proyecto** según el sistema operativo:

**Linux / macOS / WSL:**
```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem \
  -out    certs/cert.pem \
  -subj   "/C=AR/ST=Local/L=Local/O=Zapping/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"
```

**Windows — Git Bash** (viene con [Git for Windows](https://git-scm.com/download/win)):
```bash
MSYS_NO_PATHCONV=1 openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem \
  -out    certs/cert.pem \
  -subj   "/C=AR/ST=Local/L=Local/O=Zapping/CN=localhost" \
  -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"
```

**Windows — sin Git Bash ni WSL** (usando Docker directamente):
```powershell
docker run --rm -v "${PWD}/certs:/certs" alpine/openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /certs/key.pem -out /certs/cert.pem -subj "/C=AR/ST=Local/L=Local/O=Zapping/CN=localhost" -addext "subjectAltName=IP:127.0.0.1,DNS:localhost"
```

Genera `certs/cert.pem` y `certs/key.pem`. El navegador mostrará advertencia de seguridad la primera vez — aceptar y continuar.

### Paso 2 — Variables de entorno

```bash
cp .env.example .env
```

Editar `.env` y cambiar al menos:

```env
JWT_SECRET=una-clave-secreta-larga-y-aleatoria
MYSQL_ROOT_PASSWORD=rootpassword
MYSQL_PASSWORD=apppassword
```

El resto de valores funciona tal como está para desarrollo local.

### Paso 3 — Segmentos de video

Los segmentos deben estar en la carpeta `segments/` con la siguiente estructura:

```
segments/
└── big_bug_bunny/
    ├── segment.m3u8          ← playlist original del contenido
    ├── segment0.ts
    ├── segment1.ts
    └── ... (segment63.ts)
```

El archivo `segment.m3u8` es el índice que el sistema lee al arrancar para conocer los nombres y duraciones de los segmentos. Debe tener el formato HLS estándar:

```
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:10.000000,
segment0.ts
#EXTINF:10.000000,
segment1.ts
...
#EXT-X-ENDLIST
```

> Los segmentos de prueba se descargan desde el link provisto en la consigna de la prueba técnica.

### Paso 4 — Base de datos

Los scripts SQL en `database/init_db/` son ejecutados automáticamente por MySQL al arrancar por primera vez, en orden alfabético:

| Archivo | Qué hace |
|---|---|
| `00_schema.sql` | Crea las tablas `users`, `streams_metadata`, `stream_segments` |
| `01_sp_register_user.sql` | Stored procedure de registro con validación de email duplicado |
| `02_sp_get_user_by_email.sql` | Stored procedure de login |
| `03_seed_streams.sql` | Inserta los dos streams iniciales con sus `started_at` |

**No hay que ejecutarlos manualmente.** Docker los aplica desde el volumen `./database/init_db:/docker-entrypoint-initdb.d`.

> Si ya existe la carpeta `database/mysql_data/` con datos de un esquema anterior, borrarla primero:
> ```bash
> rm -rf database/mysql_data
> ```

### Paso 5 — Levantar

```bash
docker compose up --build -d
```

El orden de arranque es:
1. `db` (MySQL) — espera healthcheck
2. `nginx` — espera healthcheck (verifica que el m3u8 sea accesible)
3. `app` (Go) — arranca cuando DB y nginx están sanos; lee los streams de DB, parsea el m3u8, siembra los segmentos en la tabla `stream_segments`
4. `frontend`, `gateway`, `phpmyadmin`

### URLs disponibles

| URL | Servicio |
|---|---|
| `https://localhost:3000` | Frontend (registro, login, player) |
| `https://localhost:8080/api/` | API REST |
| `https://localhost:8081` | phpMyAdmin |

---

## 9. Agregar un nuevo stream

### Qué implica

El sistema necesita tres cosas para un nuevo stream:
1. Los archivos `.ts` y su `m3u8` en el media server (nginx)
2. Un registro en `streams_metadata`
3. Reinicio del servicio Go para que `LoadAll` detecte el nuevo stream

### Paso a paso

#### 1. Agregar los segmentos

Se puede usar el siguiente contenido de prueba para agregar un segundo canal:
[Descargar segmentos de prueba](https://drive.google.com/file/d/18Lso52cKZybn9FTa9nHamErDKPddCoji/view?usp=sharing)

Crear una carpeta dentro de `segments/` con los archivos del nuevo stream:

```
segments/
└── mi_nuevo_canal/
    ├── segment.m3u8      ← nombre obligatorio, el servicio Go lo busca exactamente así
    ├── segment0.ts
    ├── segment1.ts
    └── ...
```

> **Importante:** el archivo de playlist **debe llamarse `segment.m3u8`** sin excepción. El servicio Go construye la URL `http://nginx/segments/<segment_path>/segment.m3u8` al arrancar. Si el archivo tiene otro nombre (ej. `output.m3u8`, `index.m3u8`), el stream será ignorado con un error 404 y `stream_segments` quedará vacío para ese canal.

El nombre de la carpeta (`mi_nuevo_canal`) será el `segment_path` en la base de datos.

#### 2. Reconstruir el media server

El nginx interno copia los segmentos en tiempo de build (`COPY segments ...`), por lo que hay que reconstruirlo:

```bash
docker compose build nginx
docker compose up -d nginx
```

#### 3. Insertar el stream en la base de datos

Conectarse a phpMyAdmin en `https://localhost:8081` o ejecutar directamente:

```bash
docker exec -it livestream_db mysql -u appuser -papppassword livestream_app
```

```sql
INSERT INTO streams_metadata (title, description, segment_path, started_at, is_active)
VALUES (
    'Mi Nuevo Canal',
    'Descripción del canal',
    'mi_nuevo_canal',
    NOW(),
    TRUE
);
```

El valor de `started_at` define desde qué punto en el tiempo "arrancó" el stream. Usar `NOW()` hace que el stream empiece desde el segmento 0. Si se quiere que el canal aparezca en un punto intermedio (para diferenciarlo de otros), sumar o restar minutos:

```sql
-- Canal que "lleva 5 minutos" corriendo al momento de crearlo
started_at = DATE_SUB(NOW(), INTERVAL 5 MINUTE)
```

#### 4. Reiniciar el servicio Go

`LoadAll` solo corre al arrancar. Para que el nuevo stream sea detectado:

```bash
docker compose restart app
```

Al reiniciar, el servicio:
1. Lee `streams_metadata` desde DB → encuentra el nuevo registro
2. Hace `GET http://nginx/segments/mi_nuevo_canal/output.m3u8`
3. Parsea el m3u8 e inserta los segmentos en `stream_segments`
4. Registra el `streamMeta` en memoria (~140 bytes)

Desde ese momento el nuevo canal aparece en `GET /api/streams` y es reproducible en el player.

#### 5. Verificar

```bash
# Ver los streams activos en DB
docker exec -it livestream_db mysql -u appuser -papppassword livestream_app \
  -e "SELECT id, title, segment_path, started_at FROM streams_metadata WHERE is_active = TRUE;"

# Ver los segmentos sembrados para el nuevo stream (reemplazar ID)
docker exec -it livestream_db mysql -u appuser -papppassword livestream_app \
  -e "SELECT COUNT(*), MIN(position), MAX(position) FROM stream_segments WHERE stream_id = <ID>;"

# Ver logs del servicio Go
docker logs livestream_app --tail 20
```
