# Diagnóstico de APDU NFC en CR100

Las herramientas permiten comparar la APDU DESFire con la trama de transporte
CR100. En los ensayos realizados con este SDK y lector, una lectura segura de
archivo 4 de 63 bytes respondió correctamente y una de 64 bytes produjo una
trama completa con carga APDU vacía (`4D00072417100000010068`). Esta observación
no establece un límite universal para otros firmwares, tarjetas o modos seguros.
Los ensayos de lectura usan selección, autenticación y ReadData; no actualizan
archivos ni hacen commits en la tarjeta.

## Lectura con clave de tarjeta

`DESFIRE_TEST_KEY` debe contener una clave de pruebas de 16 bytes en hexadecimal.
La clave se proporciona al ejecutar el comando; no se guarda en este repositorio.

```sh
go run ./cmd/testApduDesfire \
  -key "$DESFIRE_TEST_KEY" -slot 4 -readlen04 63 \
  -apdu-timeout 60 -apdu-serial-timeout 3s

go run ./cmd/testApduDesfire \
  -key "$DESFIRE_TEST_KEY" -slot 4 -readlen04 64 \
  -apdu-timeout 60 -apdu-serial-timeout 3s
```

`-slot` selecciona la llave de la aplicación DESFire. `-apdu-timeout` conserva el
valor numérico enviado al SDK; no debe confundirse con el plazo del puerto serie.
`-poweron-nfc` permite probar el flujo alternativo del fabricante. Las anotaciones
muestran las APDU y las tramas; las capturas reales se mantienen fuera de Git.

## Lectura con SAM

El ejemplo de una lectura con SAM omite autenticación del host cuando `-keysam`
está vacío. Si se proporciona, debe contener exactamente 16 bytes en hexadecimal.
La llave offline del SAM realiza el cifrado/descifrado de autenticación de tarjeta.

```sh
go run ./cmd/testapduDesfirewithsam -port /dev/ttyACM0 -keyind 39
```

El ejemplo conserva la selección de llave de tarjeta 3 y los modos MAC de sus
lecturas. Para reproducir el recorrido usado en los ensayos de recuperación,
incluidas lectura completa y retirada de tarjeta, se recomienda el probe:

```sh
go run ./cmd/testcr100recovery -port /dev/ttyACM0 \
  -sam-reader ACR1252 -sam-entry 39 -card-key 4 -duration 30s
```

Este probe verifica las lecturas de archivos y conserva un guard que rechaza
comandos de escritura. El SDK mantiene el comportamiento histórico de
`NewReaderWithDevice`; la política nueva se activa por instancia con
`EnableReadRecovery` o `NewReaderWithReadRecovery`. En appfare 1.2.22_test2 se
activa por defecto para los tipos runtime `cr100` y `cr100-pc`.

## Transporte C independiente

```sh
make -C repro
./repro/dspread_apdu_repro --help
./repro/dspread_apdu_repro -p /dev/ttyACM0 --apdu 5A010000
```

`--stdin-apdu` permite enviar secuencias interactivas; `--poweron-nfc` cambia el
flujo de activación. El programa C transmite las APDU que se le indiquen, incluidas
escrituras si se le proporcionan explícitamente. No genera por sí mismo una
sesión DESFire autenticada ni calcula sus MAC.

Para ARM se debe cargar el SDK/sysroot de destino y ejecutar `make -C repro arm`.
El destino ARM fuerza el reenlace aunque exista un ejecutable AMD64. Añade los
callbacks externos exigidos por ese archivo del SDK; si se alcanza una operación
RSA ajena al transporte NFC, el reproductor termina explícitamente.
El ejecutable generado, las claves, `.env` y las capturas quedan fuera de Git.
