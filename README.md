# Telemetry

Ingesta generada/MQTT, MongoDB, consulta de consumo autorizada, escenarios y outbox persistente.

DDD: application/internal, domain/model y domain/services, infrastructure, interfaces/acl e interfaces/rest.
La base es exclusiva de este contexto. Consultas de configuración y colaboración entre servicios usan RabbitMQ RPC;
los cambios pasan por outbox y los consumidores confirman después de procesar.

El Dockerfile compila y ejecuta sus pruebas. Iniciar desde la raíz de orquestación con .env privado.

[Funcionalidad y arranque](../README.md) · [Guía RabbitMQ](../docs/rabbitmq-guide.md) · [Contratos](../docs/contracts.md).

## Historial simulado para consumo mensual

POST simulation/{unitId} con {action:"seed"} genera cuatro muestras por día y
medidor activo, desde el primer día del mes anterior hasta ayer inclusive (Lima).
No incluye horas futuras de hoy. El propietario puede usarlo desde Resumen →
Datos de demostración → Generar historial y mes anterior, o Preparar demostración.
Repetir no duplica los mismos slots/medidores; conserva lecturas y pagos existentes.
Para clientes que eligen un rango fijo, days acepta 1–62; omitido o cero usa el rango
calendario. Responde 202; consultar seeding/completed/total/error hasta terminar.
El consumo pasa por Mongo/outbox/RabbitMQ/Analytics antes de calcular la cotización.
No se crea un pago hasta que el usuario pulse Pagar consumo; órdenes ya emitidas
conservan su importe aunque después se añadan lecturas.
