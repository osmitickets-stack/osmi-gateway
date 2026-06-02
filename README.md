Las Reglas de Oro de Esta Arquitectura (Para que no la rompamos)
Regla #1: El Proto es la Ley. La definición de la API REST para el 90-95% de los casos (todos los CRUD de eventos, tickets, clientes) vive en osmi-protobuf. No crearemos nuevos endpoints REST manuales para operaciones de negocio estándar.

Regla #2: Middleware para Todo lo Transversal. Autenticación, logging, rate limiting, métricas. Todo se aplica de una vez y para todos los endpoints (tanto los automáticos como los manuales) en la capa de middleware.

Regla #3: Handlers Manuales Solo para Excepciones. Los únicos endpoints que merecen un handler manual son aquellos que:

No son operaciones CRUD sobre una entidad (ej. /login).

Requieren interactuar con el mundo exterior (ej. /webhooks/stripe).

Son puramente operativos del gateway (ej. /health, /metrics).

Regla #4: El Cliente gRPC es un Detalle de Implementación. La lógica de cómo conectarse a osmi-server (pool de conexiones, reintentos) está encapsulada en internal/grpc/client/. Los handlers manuales usan una interfaz simple de este cliente, no saben si es gRPC, HTTP o lo que sea.

Regla #5: Los Errores se Mapean, No se Filtran. El grpc/errors/mapper.go es crucial. Un error codes.NotFound de gRPC se convierte en un 404 Not Found HTTP con un mensaje amigable. Nunca dejamos que un error crudo de gRPC llegue al cliente.

gateway debe tener solo 4 responsabilidades:

1️⃣ recibir HTTP
2️⃣ aplicar middleware
3️⃣ convertir a gRPC
4️⃣ devolver JSON

osmi-gateway/
├── cmd/
│   └── main.go                      # Punto de entrada ÚNICO. Inicializa todo.
│
├── internal/                        # Código privado (NO importable desde fuera)
│   │
│   ├── config/                      # Configuración de la aplicación
│   │   └── config.go                # Carga desde env vars o archivos (ej. con viper)
│   │
│   ├── grpc/                        # Conexión con el mundo gRPC (el "backend")
│   │   ├── connection.go                  #  gRPC reutilizables y con pool de conexiones
│   │   │   └──                     # Gestiona las conexiones a osmi-server
│   │   └──    error_mapper.go                 # Mapeo de errores gRPC a HTTP
│   │       └──          # Convierte códigos gRPC a status codes HTTP
│   │
│   ├── middleware/                  # La "CAPA DE SEGURIDAD Y CONTROL" del recepcionista
│   │   ├── cors.go                  # CORS (Cross-Origin Resource Sharing)
│   │   ├── logging.go               # Logging estructurado de cada petición (Request ID, método, path, duración)
│   │   ├── recovery.go              # Recuperación de panics (para no caer el servidor)
│   │   ├── request_id.go            # Añade/Propaga un ID único por petición (para trazabilidad)
│   │   ├── auth.go                  # Middleware de autenticación JWT (valida tokens)
│   │   ├── rate_limit.go            # Rate limiting por IP o por usuario (ej. con Token Bucket)
│   │   └── metrics.go               # Middleware para exponer métricas (Prometheus)
│   │
│   ├── handlers/                    # La "RECEPCIÓN PRIVADA" para casos especiales (APROX. 5-10% de los endpoints)
│   │   ├── auth/                    # Endpoints de autenticación (NO van por gRPC directo)
│   │   │   └── auth_handler.go      # POST /login, POST /refresh, POST /logout
│   │   ├── health/                  # Endpoints de salud y estado
│   │   │   └── health_handler.go    # GET /health, GET /ready
│   │   ├── webhook/                 # Endpoints para recibir webhooks de terceros (Stripe, etc.)
│   │   │   └── webhook_handler.go   # POST /webhooks/stripe
│   │   ├── metrics/                 # 

│   │
│   └── observability/               #
│   │   ├── metrics.go
│   │   ├── tracing.go
│   │   ├── logging.go
│   │
│   └── server/                      # Montaje del servidor HTTP
│       └── server.go                # Configura el router, aplica middleware y arranca
│
├── pkg/                             # Código público (potencialmente reutilizable)
│   └── utils/                       # Utilidades muy genéricas
│       ├── converters.go            # Conversiones de tipos (si son necesarias)
│       └── validators.go            # Validadores de formato (email, UUID) - OJO: No reglas de negocio
│
├── test/                            # Pruebas
│   ├── integration/
│   │   └── gateway_test.go
│   └── unit/
│
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
└── README.md


6. Flujo real dentro del gateway
Así funcionará una petición.

POST /customers
flujo:

HTTP request
     │
middleware chain
     │
gRPC-Gateway
     │
proto mapping
     │
gRPC call
     │
osmi-server
     │
response
     │
HTTP JSON

Tu gateway ni siquiera toca el negocio.

7. Middleware profesional (orden correcto)
Es MUY importante el orden correcto:

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
