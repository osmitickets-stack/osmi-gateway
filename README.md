# OSMI Gateway

`osmi-gateway` es la **capa de entrada HTTP de OSMI**.

Su responsabilidad es recibir tráfico HTTP, aplicar middleware transversal, traducir las solicitudes hacia gRPC y devolver respuestas HTTP/JSON al cliente.

El Gateway **no contiene lógica de negocio**. Las reglas comerciales pertenecen a `osmi-server`, mientras que la definición principal de contratos vive en `osmi-protobuf`.

---

## Responsabilidad del módulo

`osmi-gateway` debe tener solamente cuatro responsabilidades principales:

1. recibir HTTP;
2. aplicar middleware;
3. convertir HTTP ↔ gRPC;
4. devolver JSON.

Conceptualmente:

```text
Cliente HTTP
    │
    ▼
osmi-gateway
    │
    ├── middleware
    ├── routing
    ├── REST ↔ gRPC
    └── mapping de errores
    │
    ▼
osmi-server
```

El Gateway no debe decidir reglas de negocio como:

- si una orden puede venderse;
- si un ticket puede convertirse en `SOLD`;
- si un pago es válido;
- cómo se modifica inventario;
- cómo se completa una orden.

Esas decisiones pertenecen al backend de aplicación.

---

# Reglas de oro de la arquitectura

## Regla 1 — El Proto es la ley

La definición de la API REST para la gran mayoría de operaciones estándar vive en `osmi-protobuf`.

Eso incluye principalmente:

- eventos;
- tickets;
- clientes;
- órdenes;
- pagos;
- operaciones CRUD estándar.

No se deben crear endpoints REST manuales para operaciones normales de negocio cuando ya pueden expresarse mediante Protobuf + gRPC-Gateway.

```text
osmi-protobuf
     │
     ├── mensajes
     ├── servicios
     ├── rutas HTTP
     └── contratos
          │
          ▼
     osmi-gateway
```

El Gateway implementa y expone el contrato; no inventa una API paralela.

---

## Regla 2 — Middleware para todo lo transversal

Toda preocupación transversal debe resolverse en middleware.

Ejemplos:

- Request ID;
- recovery;
- logging;
- CORS;
- rate limiting;
- autenticación;
- autorización transversal;
- métricas;
- observabilidad.

El objetivo es evitar lógica repetida en cada endpoint.

---

## Regla 3 — Handlers manuales sólo para excepciones

Un handler manual debe existir únicamente cuando el endpoint no encaja razonablemente en el flujo estándar generado por gRPC-Gateway.

Casos legítimos:

### Autenticación especial

```text
/login
```

cuando requiere un flujo HTTP específico que no es simplemente un CRUD.

### Integraciones externas

```text
/v1/webhooks/stripe
```

porque Stripe envía una petición HTTP firmada que debe recibirse y reenviarse con el payload correspondiente.

### Operación del Gateway

```text
/health
/metrics
```

porque pertenecen al propio proceso del Gateway.

La excepción no debe convertirse en la regla.

---

## Regla 4 — El cliente gRPC es un detalle de implementación

La lógica de conexión con `osmi-server` debe permanecer encapsulada.

Los handlers y capas superiores no deben conocer detalles como:

- establecimiento de conexiones;
- keepalive;
- reconexión;
- retries;
- configuración del transporte;
- detalles internos de gRPC.

Conceptualmente:

```text
Handler / Router
      │
      ▼
interfaz cliente
      │
      ▼
internal/grpc/...
      │
      ▼
osmi-server
```

Esto evita acoplar los handlers directamente a detalles de transporte.

---

## Regla 5 — Los errores se mapean, no se filtran

Los errores internos de gRPC no deben llegar crudos al cliente HTTP.

Ejemplo:

```text
gRPC codes.NotFound
        │
        ▼
mapper de errores
        │
        ▼
HTTP 404
```

Otros ejemplos conceptuales:

```text
InvalidArgument   → 400
Unauthenticated   → 401
PermissionDenied  → 403
NotFound          → 404
AlreadyExists     → 409
ResourceExhausted → 429
Internal          → 500
```

La respuesta HTTP debe ser coherente y entendible para el cliente.

Nunca se deben exponer detalles internos innecesarios del transporte gRPC.

---

## Flujo real de una petición

Ejemplo:

```text
POST /customers
```

El Gateway no toca la lógica comercial.

```text
HTTP Request
     │
     ▼
Middleware Chain
     │
     ▼
gRPC-Gateway
     │
     ▼
Proto Mapping
     │
     ▼
gRPC Call
     │
     ▼
osmi-server
     │
     ▼
gRPC Response
     │
     ▼
HTTP / JSON
```

---

## Orden del middleware

El orden esperado del middleware es:

```text
RequestID
   ↓
Recovery
   ↓
Logging
   ↓
CORS
   ↓
RateLimit
   ↓
Auth
   ↓
Router
```

### Request ID

Genera o propaga un identificador por solicitud para trazabilidad.

### Recovery

Evita que un panic derribe el proceso y transforma fallos inesperados en respuestas controladas.

### Logging

Registra la petición utilizando el Request ID disponible.

### CORS

Controla qué orígenes web pueden consumir la API.

### Rate Limit

Protege endpoints contra abuso y tráfico excesivo.

### Auth

Valida autenticación cuando la ruta lo requiere.

### Router

Entrega la solicitud al endpoint generado o handler manual correspondiente.

---

## gRPC-Gateway

La mayoría de las rutas HTTP son generadas a partir de las anotaciones definidas en Protobuf.

Ejemplo conceptual:

```proto
rpc GetEvent(GetEventRequest) returns (EventResponse) {
  option (google.api.http) = {
    get: "/v1/events/{public_id}"
  };
}
```

Esto permite mantener sincronizados:

```text
Contrato gRPC
     +
Contrato HTTP
     +
Mensajes
```

desde una única definición.

---

## Endpoints manuales

Los endpoints manuales deben permanecer pocos y claramente justificados.

Ejemplos actuales o esperados:

```text
/health
/v1/webhooks/stripe
```

Los endpoints CRUD y de negocio ordinario deben preferir siempre el contrato Protobuf.

---

## Stripe webhook

El Gateway recibe el webhook HTTP de Stripe y lo transfiere al backend correspondiente.

Conceptualmente:

```text
Stripe
   │
   ▼
POST /v1/webhooks/stripe
   │
   ▼
osmi-gateway
   │
   ▼
osmi-server
   │
   ├── valida firma
   ├── controla idempotencia
   ├── actualiza pago
   └── ejecuta fulfillment
```

El Gateway no debe decidir si el pago es válido ni modificar órdenes o tickets directamente.

---

## Autenticación

La autenticación se aplica como una preocupación transversal.

Conceptualmente:

```text
Request
   │
   ▼
Auth Middleware
   │
   ├── ruta pública → continuar
   │
   └── ruta protegida
          │
          ├── token válido → continuar
          └── token inválido → rechazar
```

Redis puede apoyar funciones como blacklist o revocación, pero no debe convertirse en la autoridad primaria de identidad.

---

## Redis

El Gateway utiliza Redis para capacidades transversales cuando corresponde, por ejemplo:

- blacklist de tokens;
- rate limiting;
- información efímera operativa.

La lógica de negocio persistente no pertenece a Redis.

---

## Health check

El Gateway expone un endpoint operativo:

```text
GET /health
```

Su propósito es permitir que Docker, balanceadores y sistemas de monitoreo determinen si el proceso está disponible.

La disponibilidad del proceso no debe confundirse con la salud completa de todas las dependencias.

---

## CORS

CORS se gestiona centralmente en middleware.

En producción los orígenes permitidos deben definirse explícitamente.

Ejemplo conceptual:

```env
CORS_ORIGINS=https://www.myosmi.com,https://myosmi.com
```

No se deben abrir orígenes indiscriminadamente en producción.

---

## Comunicación con `osmi-server`

Dentro de Docker Compose, el Gateway se comunica mediante el nombre DNS interno del servicio.

Ejemplo:

```env
GRPC_SERVER_ADDR=server:50051
```

Flujo:

```text
gateway
   │
   ▼
server:50051
   │
   ▼
osmi-server
```

El puerto gRPC no necesita exponerse públicamente para que ambos servicios se comuniquen dentro de la red Docker.

---

## Puertos

| Servicio | Puerto |
|---|---:|
| HTTP Gateway | `8083` |

En producción, el tráfico público llega normalmente a través de Nginx:

```text
Internet
   │
   ▼
Cloudflare
   │
   ▼
Nginx
   │
   ▼
osmi-gateway:8083
```

---

## Variables de entorno

### Aplicación

```env
ENVIRONMENT=
HTTP_PORT=
LOG_LEVEL=
```

### gRPC

```env
GRPC_SERVER_ADDR=
```

### Redis

```env
REDIS_URL=
REDIS_PASSWORD=
REDIS_DB=
```

### Seguridad

```env
JWT_SECRET_KEY=
CORS_ORIGINS=
```

Las variables reales y secretos no deben versionarse.

---

## Ejecución local

Desde el repositorio:

```bash
go test ./...
go build ./...
```

Construcción Docker desde el directorio padre:

```bash
docker build -t osmi-gateway:local-corrected ./osmi-gateway
```

Para desarrollo local puede utilizarse el override:

```text
compose.local-corrected.yml
```

Ese archivo es exclusivo del entorno local.

No debe copiarse a EC2.

---

## Despliegue

En producción, el Gateway utiliza la imagen publicada en GHCR.

Conceptualmente:

```text
Git Push
   │
   ▼
GitHub
   │
   ▼
CI/CD
   │
   ▼
Docker Build
   │
   ▼
GHCR
   │
   ▼
EC2
```

Imagen de producción:

```text
ghcr.io/osmitickets-stack/osmi-gateway:latest
```

EC2 no debe utilizar la imagen local `osmi-gateway:local-corrected`.

---

## Arquitectura de producción

```text
                 INTERNET
                    │
                    ▼
                Cloudflare
                    │
                    ▼
                  Nginx
                    │
                    ▼
              osmi-gateway
                    │
                    ▼
                 gRPC
                    │
                    ▼
               osmi-server
                    │
                    ▼
               PostgreSQL
```

Para webhooks:

```text
Stripe
   │
   ▼
Cloudflare / Nginx
   │
   ▼
osmi-gateway
   │
   ▼
osmi-server
```

---

## Qué pertenece al Gateway

```text
✅ entrada HTTP
✅ routing
✅ gRPC-Gateway
✅ middleware
✅ CORS
✅ Request ID
✅ logging HTTP
✅ rate limiting
✅ autenticación transversal
✅ mapping HTTP ↔ gRPC
✅ mapping de errores
✅ webhook HTTP de integraciones externas
✅ health checks
✅ métricas operativas
```

## Qué no pertenece al Gateway

```text
❌ reglas de negocio
❌ cálculo de inventario
❌ transición RESERVED → SOLD
❌ lógica de órdenes
❌ lógica de pagos
❌ persistencia PostgreSQL de negocio
❌ envío de boletos
❌ lógica de email
❌ decisión de si un pago es válido
❌ definición duplicada de contratos REST
```

---

## Relación con otros módulos

```text
osmi-protobuf
    → contratos gRPC/HTTP

osmi-gateway
    → entrada HTTP y middleware

osmi-server
    → lógica de negocio

osmi-db
    → esquema PostgreSQL

osmi-front
    → aplicación web
```

Cada módulo mantiene su propio README y responsabilidad.

---

## Principios de mantenimiento

- El Proto es la fuente principal del contrato.
- No duplicar rutas REST manualmente.
- El Gateway no implementa lógica comercial.
- Las preocupaciones transversales viven en middleware.
- Los handlers manuales deben ser excepcionales.
- Los errores internos se transforman antes de llegar al cliente.
- La conexión gRPC permanece encapsulada.
- Los secretos sólo llegan mediante variables de entorno.
- Las rutas públicas deben estar explícitamente definidas.
- La infraestructura de producción no debe depender de overrides locales.

---

## Estado actual

El Gateway ha sido validado en local y EC2 como parte del flujo:

```text
Frontend
   │
   ▼
Gateway
   │
   ▼
Server
   │
   ▼
PostgreSQL
```

También se ha comprobado:

```text
Nginx
   │
   ▼
Gateway /health    → 200

Nginx
   │
   ▼
Gateway /v1/events → 200
```

y el flujo de Stripe utiliza:

```text
POST /v1/webhooks/stripe
```

con respuestas HTTP exitosas durante las pruebas E2E.

---

## Trabajo pendiente

Antes de considerar esta capa completamente endurecida:

1. revisar de forma final las rutas públicas y protegidas;
2. consolidar la política de autenticación por endpoint;
3. validar rate limiting en producción;
4. mejorar observabilidad y métricas;
5. revisar healthchecks de dependencias;
6. revisar configuración de keepalive gRPC para evitar eventos como `too_many_pings`;
7. mantener tests de integración HTTP/gRPC;
8. verificar mapping consistente de todos los errores gRPC.

---

## Estructura
```bash
osmi-gateway/
├── .github/
│   └── workflows/
│   │   ├── ci.yml
│   │   ├── deploy.yml 
│   │   └── docker.yml
├── cmd/
│   └── main.go                      # Punto de entrada ÚNICO. Inicializa todo.
├── internal/                        # Código privado (NO importable desde fuera)
│   ├── cache/                      # Configuración de la aplicación
│   │   └── redis.go  
│   ├── config/                      # Configuración de la aplicación
│   │   └── config.go                # Carga desde env vars o archivos (ej. con viper)
│   ├── grpc/                        # Conexión con el mundo gRPC (el "backend")
│   │   ├── auth_interceptor.go
│   │   ├── connection.go                  #  gRPC reutilizables y con pool de conexiones
│   │   └──    error_mapper.go                 # Mapeo de errores gRPC a HTTP
│   ├── handlers/                    # La "RECEPCIÓN PRIVADA" para casos especiales (APRX. 10% de los endpoints)
│   │   ├── auth/                    # Endpoints de autenticación (NO van por gRPC directo)
│   │   │   └── auth_handler.go      # POST /login, POST /refresh, POST /logout
│   │   ├── health/                  # Endpoints de salud y estado
│   │   │   └── health_handler.go    # GET /health, GET /ready
│   │   ├── webhook/                 # Endpoints para recibir webhooks de terceros (Stripe, etc.)
│   │   │   └── webhook_handler.go   # POST /webhooks/stripe
│   │   ├── metrics/                 # 
│   │   ├── webhook/ 
│   │   ├── protected_handler.go 
│   ├── middleware/                  # La "CAPA DE SEGURIDAD Y CONTROL" del recepcionista
│   │   ├── auth.go                  # Middleware de autenticación JWT (valida tokens)
│   │   ├── cors.go                  # CORS (Cross-Origin Resource Sharing)
│   │   ├── logging.go               # Logging estructurado de cada petición (Request ID, método, path, duración)
│   │   └── metrics.go               # Middleware para exponer métricas (Prometheus)
│   │   ├── rate_limit.go            # Rate limiting por IP o por usuario (ej. con Token Bucket)
│   │   ├── recovery.go              # Recuperación de panics (para no caer el servidor)
│   │   ├── request_id.go            # Añade/Propaga un ID único por petición (para trazabilidad)
│   └── observability/               #
│   │   ├── logging.go
│   │   ├── metrics.go
│   │   └── tracing.go
│   └── server/                      # Montaje del servidor HTTP
│       └── server.go                # Configura el router, aplica middleware y arranca
├── pkg/                             # Código público (potencialmente reutilizable)
│   └── utils/                       # Utilidades muy genéricas
│       ├── converters.go            # Conversiones de tipos (si son necesarias)
│       ├── helpers.go
│       └── validators.go            # Validadores de formato (email, UUID) - OJO: No reglas de negocio
├── test/                            # Pruebas
│   ├── integration/
│   │   └── gateway_test.go
│   └── unit/
├── .env
├── .env.example
├── .gitignore
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Autor

**Francisco David Zamora Urrutia** | Fullstack Developer · Systems Engineer
