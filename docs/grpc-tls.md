# Conexión autenticada con osmi-server

El cliente gRPC exige TLS 1.3, una CA privada de confianza, certificado y clave de cliente y el nombre TLS del Server. No hay fallback a texto plano. La configuración se carga antes de abrir conexiones a Redis.

Variables obligatorias de archivos: `GRPC_TLS_CA_FILE`, `GRPC_TLS_CERT_FILE`, `GRPC_TLS_KEY_FILE`. El certificado debe ser de cliente y contener SAN DNS exacto `osmi-gateway` o el nombre autorizado en Server. `GRPC_TLS_SERVER_NAME` vale `osmi-server` por defecto y debe corresponder al certificado del Server, aunque `GRPC_SERVER_ADDR` utilice una IP o `localhost`.

Copie únicamente las variables necesarias de `.env.grpc-tls.example` a su entorno, conservando los demás ajustes de JWT, Redis y HTTP. No versionar certificados privados. Al faltar certificados, el Gateway devuelve error de configuración; al no poder verificar al Server, la llamada gRPC falla.

El JWT debe ser HS256 con expiración y un `user_id` UUID público canónico. Un error del Redis de revocación configurado rechaza la autenticación. El Gateway nunca permite que encabezados HTTP arbitrarios sustituyan la identidad interna.

La guía completa, generación local de certificados, arranque, política de emisión, rotación y limitaciones están en `osmi-server/docs/gateway-server-security.md` del repositorio hermano. El generador local se ejecuta desde `osmi-server` con `bash scripts/create-dev-grpc-certs.sh ../.local/grpc-tls`. Los ejemplos presuponen ambos repositorios bajo `osmi-back`; las claves se generan fuera de ellos.

Este cambio requiere configurar ambos servicios antes de reiniciarlos. No modifica el despliegue existente ni configura certificados productivos automáticamente.
