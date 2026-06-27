with open('internal/routingpolicy/policy_test.go', 'r', encoding='utf-8') as f:
    content = f.read()

old = '\tpolicy, err := svc.SetServicePolicy(input)\n\tif err != nil {\n\t\tt.Fatalf("create: %v", err)\n\t}\n\n\tfetched, _ := svc.GetServicePolicy("svc_rt")'

new = '\t_, err := svc.SetServicePolicy(input)\n\tif err != nil {\n\t\tt.Fatalf("create: %v", err)\n\t}\n\n\tfetched, _ := svc.GetServicePolicy("svc_rt")'

if old not in content:
    # Try finding the location
    idx = content.find('SetServicePolicy(input)')
    print(f'Found at index {idx}')
    print(repr(content[idx-40:idx+200]))
else:
    content = content.replace(old, new, 1)
    with open('internal/routingpolicy/policy_test.go', 'w', encoding='utf-8') as f:
        f.write(content)
    print('OK')
