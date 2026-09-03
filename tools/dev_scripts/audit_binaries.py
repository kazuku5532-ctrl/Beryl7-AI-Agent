import os
import struct
import hashlib

def inspect_elf_binary(path):
    if not os.path.exists(path):
        print(f"FILE NOT FOUND: {path}")
        return None
        
    size = os.path.getsize(path)
    size_mib = size / (1024 * 1024)
    size_mb = size / 1_000_000
    
    with open(path, "rb") as f:
        content = f.read()
    
    sha256 = hashlib.sha256(content).hexdigest()
    magic = content[:4]
    magic_ok = (magic == b"\x7fELF")
    ei_class = content[4]  # 1=32-bit, 2=64-bit
    ei_data = content[5]   # 1=LE, 2=BE
    endian = "<" if ei_data == 1 else ">"
    
    e_type, e_machine, e_version = struct.unpack(endian + "HHI", content[16:24])
    
    machine_map = {
        183: "EM_AARCH64 (ARM 64-bit)",
        40: "EM_ARM (ARMv7 32-bit)",
        62: "EM_X86_64 (AMD 64-bit)",
        8: "EM_MIPS (MIPS)"
    }
    
    # Program headers
    if ei_class == 2:
        e_phoff = struct.unpack(endian + "Q", content[32:40])[0]
        e_phentsize, e_phnum = struct.unpack(endian + "HH", content[54:58])
        e_shoff = struct.unpack(endian + "Q", content[40:48])[0]
        e_shentsize, e_shnum = struct.unpack(endian + "HH", content[58:62])
        e_shstrndx = struct.unpack(endian + "H", content[62:64])[0]
    else:
        e_phoff = struct.unpack(endian + "I", content[28:32])[0]
        e_phentsize, e_phnum = struct.unpack(endian + "HH", content[42:46])
        e_shoff = struct.unpack(endian + "I", content[32:36])[0]
        e_shentsize, e_shnum = struct.unpack(endian + "HH", content[46:50])
        e_shstrndx = struct.unpack(endian + "H", content[50:52])[0]
        
    pt_interp = False
    for i in range(e_phnum):
        ph_offset = e_phoff + i * e_phentsize
        p_type = struct.unpack(endian + "I", content[ph_offset:ph_offset+4])[0]
        if p_type == 3:  # PT_INTERP
            pt_interp = True
            break
            
    sections = []
    if e_shnum > 0 and e_shoff > 0:
        str_sh_offset = e_shoff + e_shstrndx * e_shentsize
        if ei_class == 2:
            sh_offset = struct.unpack(endian + "Q", content[str_sh_offset+24:str_sh_offset+32])[0]
            sh_size = struct.unpack(endian + "Q", content[str_sh_offset+32:str_sh_offset+40])[0]
        else:
            sh_offset = struct.unpack(endian + "I", content[str_sh_offset+16:str_sh_offset+20])[0]
            sh_size = struct.unpack(endian + "I", content[str_sh_offset+20:str_sh_offset+24])[0]
        shstrtab = content[sh_offset:sh_offset+sh_size]
        
        for i in range(e_shnum):
            s_offset = e_shoff + i * e_shentsize
            sh_name_idx = struct.unpack(endian + "I", content[s_offset:s_offset+4])[0]
            end = shstrtab.find(b"\x00", sh_name_idx)
            s_name = shstrtab[sh_name_idx:end].decode("utf-8", errors="ignore")
            sections.append(s_name)
            
    debug_sections = [s for s in sections if s.startswith(".debug_")]
    has_symtab = ".symtab" in sections
    
    result = {
        "path": path,
        "size_bytes": size,
        "size_mib": size_mib,
        "size_mb": size_mb,
        "sha256": sha256,
        "magic_ok": magic_ok,
        "class": "ELF64" if ei_class == 2 else "ELF32",
        "endianness": "Little Endian" if ei_data == 1 else "Big Endian",
        "type": "ET_EXEC" if e_type == 2 else ("ET_DYN (PIE)" if e_type == 3 else str(e_type)),
        "machine_code": e_machine,
        "machine_name": machine_map.get(e_machine, "UNKNOWN"),
        "statically_linked": not pt_interp,
        "stripped": len(debug_sections) == 0,
        "debug_sections": debug_sections,
        "has_symtab": has_symtab,
        "flash_budget_pass": size_mib < 16.0,
        "flash_headroom_mib": 16.0 - size_mib,
        "target_diff_mib": size_mib - 8.6
    }
    
    print("=" * 70)
    print(f"Binary: {path}")
    print(f"  Exact Size:       {size:,} bytes ({size_mib:.2f} MiB / {size_mb:.2f} MB)")
    print(f"  Flash Budget:     {'PASS' if result['flash_budget_pass'] else 'FAIL'} (< 16.0 MiB, Headroom: {result['flash_headroom_mib']:.2f} MiB)")
    print(f"  Target Budget:    Delta from ~8.6 MiB target: {result['target_diff_mib']:+.2f} MiB")
    print(f"  SHA256 Checksum:  {sha256}")
    print(f"  ELF Magic:        {'0x7fELF (Valid)' if magic_ok else 'INVALID'}")
    print(f"  Architecture:     {result['machine_name']} [Code: {e_machine}]")
    print(f"  ELF Class:        {result['class']}")
    print(f"  Endianness:       {result['endianness']}")
    print(f"  ELF Type:         {result['type']}")
    print(f"  Linking:          {'Statically linked (No PT_INTERP)' if result['statically_linked'] else 'Dynamically linked'}")
    print(f"  Sections Total:   {len(sections)}")
    print(f"  Debug Sections:   {len(debug_sections)} ({', '.join(debug_sections) if debug_sections else 'None'})")
    print(f"  Symtab:           {'.symtab present' if has_symtab else 'Stripped (No .symtab)'}")
    print(f"  Stripped Status:  {'Fully Stripped (-s -w verified)' if result['stripped'] and not has_symtab else 'Partially/Unstripped'}")
    return result

if __name__ == "__main__":
    targets = [
        "go-agent/beryl7-agent",
        "dist/beryl7-agent-linux-arm64",
        "dist/beryl7-agent-linux-armv7",
        "dist/beryl7-agent-linux-amd64"
    ]
    results = [inspect_elf_binary(t) for t in targets]
    print("=" * 70)
    print("Binary Sync Audit (go-agent/beryl7-agent vs dist/beryl7-agent-linux-arm64):")
    r0 = results[0]
    r1 = results[1]
    if r0 and r1:
        if r0["sha256"] == r1["sha256"]:
            print(f"  VERDICT: 100% BIT-FOR-BIT SYNCHRONIZED (SHA256: {r0['sha256']})")
        else:
            print(f"  MISMATCH: r0={r0['sha256']} vs r1={r1['sha256']}")
