# Роль `hw_monitor_agent`

Роль разворачивает Linux-агент HW Monitor на x86_64-дистрибутивах с systemd.
Перед установкой она получает подписанный манифест, проверяет Ed25519-подпись
встроенным публичным ключом и SHA-256 установщика. Повторный запуск не меняет
уже актуальный агент.

Не храните `ncm_handshake_key` в playbook или репозитории. Передавайте его из
Ansible Vault, AWX/Tower Credential или другого менеджера секретов.

```yaml
- hosts: monitored_linux
  become: true
  roles:
    - role: hw_monitor_agent
      vars:
        ncm_handshake_key: '{{ vault_hw_monitor_handshake_key }}'
        ncm_allowed_cidrs: '10.20.0.15/32'
        ncm_profile: auto
        ncm_release_channel: stable
```

Для внутреннего тестирования допускается `ncm_release_channel: test`. Его
нельзя использовать в инвентарях клиентов.
