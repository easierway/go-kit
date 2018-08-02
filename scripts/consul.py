#!/usr/bin/env python3

import json
import requests
import argparse
import socket
import subprocess


def local_ip():
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(('8.8.8.8', 80))
        ip = s.getsockname()[0]
    finally:
        s.close()

    return ip


def default_balance_factor_map(fm="consul/factor_map.json", consul="localhost:8500"):
    if not fm:
        fm = "consul/factor_map.json"
    if not consul:
        consul = "localhost:8500"
    url = "http://{}/v1/kv/{}?raw=true".format(consul, fm)
    res = requests.get(url)
    if res.status_code != 200:
        return {
            "m4.4xlarge": 100,
            "r4.4xlarge": 100,
            "c4.4xlarge": 100,
            "unknown": 100,
        }
    return json.loads(res.text)


def default_balance_factor(fm="consul/factor_map.json", consul="localhost:8500"):
    factor_map = default_balance_factor_map(fm, consul)
    print(factor_map)

    status, output = subprocess.getstatusoutput("/opt/aws/bin/ec2-metadata -t")
    machine = "unknown"
    if status == 0:
        machine = output.split()[1]

    if machine in factor_map:
        return factor_map[machine]

    return 100


def default_zone():
    status, output = subprocess.getstatusoutput("/opt/aws/bin/ec2-metadata -z")
    zone = "unknown"
    if status == 0:
        zone = output.split()[1]
    return zone


def hostname():
    status, output = subprocess.getstatusoutput("hostname")
    hostname = "unknown"
    if status == 0:
        hostname = output.strip()
    return hostname


def tag():
    return hostname().split(":")[0]


def payload(service, port, tags=[], interval="10s", timeout="1s",
            deregister_critical_service_after="90m",
            balance_factor=None, zone=None, fm=None, consul="localhost:8500"):
    ip = local_ip()
    if not balance_factor:
        balance_factor = str(default_balance_factor(fm, consul))
    if not zone:
        zone = default_zone()

    return {
        "ID": "{}-{}-{}".format(service, ip, port),
        "Name": service,
        "Tags": tags,
        "Address": ip,
        "Meta": {
            "balanceFactor": balance_factor,
            "zone": zone
        },
        "Port": port,
        "EnableTagOverride": False,
        "Check": {
            "Name": "check port",
            "TCP": "{}:{}".format(ip, port),
            "Interval": interval,
            "Timeout": timeout,
            "DeregisterCriticalServiceAfter": deregister_critical_service_after
        }
    }


def register(data, consul="localhost:8500"):
    url = "http://{}/v1/agent/service/register".format(consul)
    res = requests.put(url, json.dumps(data))
    if res.status_code != 200:
        print(res.status_code, res.reason)
        print(url)
    print(json.dumps(data, indent=4))


def deregister(service_id="", consul="localhost:8500"):
    url = "http://{}/v1/agent/service/deregister/{}".format(
        consul, service_id)
    res = requests.put(url)
    print(res.status_code, res.reason)
    print(url)


def services(service="", consul="localhost:8500"):
    if not service:
        url = "http://{}/v1/catalog/services".format(consul)
    else:
        url = "http://{}/v1/health/service/{}?passing=true".format(
            consul, service)
    res = requests.get(url)
    if res.status_code != 200:
        print(res.status_code, res.reason)
        print(url)
    else:
        print(res.text)


def kvput(src, dst, consul="localhost:8500"):
    url = "http://{}/v1/kv/{}".format(consul, dst)
    with open(src) as f:
        text = f.read()
        res = requests.put(url, text)
        if res.status_code != 200:
            print(res.status_code, res.reason)
            print(url)
        else:
            print(text)


def kvget(dst, consul="localhost:8500"):
    url = "http://{}/v1/kv/{}?raw=true".format(consul, dst)
    res = requests.get(url)
    if res.status_code != 200:
        print(res.status_code, res.reason)
        print(url)
    else:
        print(res.text)


def config(datacenter):
    obj = {
        "datacenter": datacenter,
        "data_dir": "/usr/local/consul/data",
        "log_level": "INFO",
        "node_name": hostname(),
        "server": False,
        "ui": True,
        "bootstrap_expect": 0,
        "bind_addr": local_ip(),
        "client_addr": "127.0.0.1",
        # "retry_join": ["provider=aws tag_key=Name tag_value={} region={}".format(tag(), default_zone())],
        "retry_interval": "3s",
        "raft_protocol": 3,
        "enable_debug": False,
        "rejoin_after_leave": True,
        "enable_syslog": False
    }
    print(json.dumps(obj, indent=4, sort_keys=True))


def main():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description="""Example:
    python3 consul.py config sg > /etc/consul/consul.json
    python3 consul.py kvput --src factor_map.json --dst consul/as/factor_map.json
    python3 consul.py kvget consul/as/factor_map.json
    python3 consul.py register as 9099 --factor-map consul/as/factor_map.json
    python3 consul.py register rs 7077 --factor-map consul/rs/factor_map.json
    python3 consul.py services rs | jq '.[] | {Service}'
""",
    )
    parser.add_argument("operation", nargs="?", type=str,
                        choices=["register", "services",
                                 "deregister", "kvget", "kvput", "config"],
                        help="operation")
    parser.add_argument("service", nargs="?", type=str,
                        help="service name or datacenter")
    parser.add_argument("port", nargs="?", type=int, help="service port")
    parser.add_argument("-i", "--interval", default="10s",
                        help="health check interval")
    parser.add_argument("-t", "--timeout", default="1s",
                        help="health check timeout")
    parser.add_argument("-r", "--deregister", default="90m",
                        help="health check DeregisterCriticalServiceAfter")
    parser.add_argument("-c", "--consul", default="localhost:8500",
                        help="consul address")
    parser.add_argument("-z", "--zone", type=str, help="zone")
    parser.add_argument("-f", "--factor", type=str, help="balance factor")
    parser.add_argument("-m", "--factor-map", type=str, default="consul/factor_map.json",
                        help="balance factor map consul path")
    parser.add_argument("-s", "--src", type=str, help="upload file source")
    parser.add_argument("-d", "--dst", type=str, help="upload file kv path")
    args = parser.parse_args()
    if args.operation == "register":
        register(
            payload(
                args.service,
                args.port,
                interval=args.interval,
                timeout=args.timeout,
                deregister_critical_service_after=args.deregister,
                balance_factor=args.factor,
                zone=args.zone,
                fm=args.factor_map,
            ),
            args.consul
        )
    elif args.operation == "services":
        services(args.service, args.consul)
    elif args.operation == "deregister":
        deregister(args.service, args.consul)
    elif args.operation == "kvput":
        kvput(args.src, args.dst, args.consul)
    elif args.operation == "kvget":
        kvget(args.dst, args.consul)
    elif args.operation == "config":
        config(args.service)
    else:
        print("not support operation [{}]".format(args.operation))


if __name__ == '__main__':
    main()
