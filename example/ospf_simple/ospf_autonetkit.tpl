ip forwarding
!
{% for interface in data.interfaces %}
interface {{ .name }}
 ip address {{ interface.ip }}/{{ interface.plen }}
!
{% endfor %}
{% if data.ospf.enabled %}
router ospf
 ospf router-id {{ .ip_loopback }}
{% for network in data.ospf.networks %}
 network {{network}} area 0
{% endfor %}
{% endif %}
!

