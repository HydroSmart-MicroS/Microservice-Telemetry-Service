# Telemetry

Ingesta generada/MQTT, MongoDB, consulta de consumo autorizada, escenarios y outbox persistente.

DDD: application/internal, domain/model y domain/services, infrastructure, interfaces/acl e interfaces/rest.
La base es exclusiva de este contexto. Consultas de configuración y colaboración entre servicios usan RabbitMQ RPC;
los cambios pasan por outbox y los consumidores confirman después de procesar.

## Operación en MAIN

En la red de Compose, el servicio escucha en `telemetry:8080` y utiliza la base `hs_telemetry`.

| Comprobación | Ruta interna |
|---|---|
| Proceso | `GET /health/live` |
| Disponibilidad | `GET /health/ready` |

Readiness verifica MongoDB Atlas, RabbitMQ y MQTT. Revisa el resultado en el [panel de MAIN](http://127.0.0.1:8099/). Si falla database, revisa la conexión TLS y la IP de salida autorizada en Atlas. Si falla mqtt, comprueba la conexión del ingestor con Mosquitto.

Construye y ejecuta las pruebas del Dockerfile desde este clon con `docker build -t hydrosmart-telemetry .`. Para la prueba conjunta, ejecuta `./scripts/Test-Local.ps1 -DatabaseMode Hosted` desde la carpeta de orquestación.

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


## Consumo total y detalle por medidor

El resumen usa GENERAL para el consumo total, el número de lecturas, el historial
diario y las fechas de primera/última lectura. Incluye los GENERAL archivados para
conservar su historial tras reemplazar un medidor. Una unidad heredada que todavía
no tiene GENERAL conserva su suma anterior hasta que el propietario lo elige.

La respuesta conserva todos los medidores en devices y su detalle diario en
meterDaily. daily contiene las filas usadas para calcular el total y
totalSource indica GENERAL o legacy_sum. Analytics, Billing y Reports usan
el mismo resumen. La simulación nueva genera las zonas dentro del volumen GENERAL
y conserva las muestras ya guardadas cuando se vuelve a solicitar el historial.

La caché de simulación recupera la entrada inactiva menos reciente al alcanzar
100 unidades. Las tareas activas, los historiales en ejecución y las pruebas
de alertas mantienen su entrada.
