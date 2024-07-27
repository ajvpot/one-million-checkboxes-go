excellent blog https://eieio.games/essays/scaling-one-million-checkboxes/

- how would i do it
-
- go process - 1 million atomic.Bools
    - manipulate it with atomic. so you dont need mutexes.
    - one process holds it all in memory easily - no scaling issue there
- app is read heavy
    - have "frontend" servers
    - they connect to the "master" server and relay updates/writes.
    - keeps the master servers bandwidth from becoming saturated.
- bandwidth costs are an issue
    - use a binary protocol
    - use the MSB of the index of the checkbox to indicate if its checked or not
