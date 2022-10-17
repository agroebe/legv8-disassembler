package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"errors"
)

/* A mapping of register indices to their special formatted names (if applicable) */
var specialRegisters = map[int]string { 16: "IP0", 17: "IP1", 28: "SP", 29: "FP", 30: "LR", 31: "XZR" }

/* Option to print raw program binary for debugging. */
var printProgram bool = false

/* Represents a single, 32-bit, LEGv8 instruction. */
type Instruction struct {
	Assembly uint32
}

/* Represents LEGv8 instruction fields in their decoded format. */
type DecodedInstruction struct {
	opcode		uint32
	rd			uint32
	rn			uint32
	rm			uint32
	shamt		uint32
	immediate	uint32
	bAddress	int16
	cbAddress	int16
	dAddress	int16
}

/* An array of formatted instructions, initially empty. */
var instructions []DecodedInstruction

/* An array of booleans that specify which instructions should receive corresponding procedure labels. */
var labels []bool

/* The index of the instruction currently being decoded. */
var instructionIndex int = 0

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run disassembler.go <Assembled LEGv8 Binary File>")
		return
	}

	inputFileName := os.Args[1]
	inputFile, err := os.Open(inputFileName)
	if err != nil {
		fmt.Println("Error opening input file", err)
		return
	}

	defer inputFile.Close() // Close input file after program execution.

	fi, err := inputFile.Stat()
	if err != nil {
		fmt.Println("Error obtaining file information", err)
		return
	}

	numInstructions := fi.Size() / 4
	instructions = make([]DecodedInstruction, numInstructions)
	labels = make([]bool, numInstructions)

	for {
		instr := Instruction{}
		err := binary.Read(inputFile, binary.BigEndian, &instr) // Read 32 bit chunks into Instruction struct.

		/* End of file encountered, break from loop. */
		if err == io.EOF {
			break
		}

		decode(instr.Assembly) // Decode each 32 bit instruction.
	}

	printAssembly() // After reading all decoded instructions into memory, print the resulting assembly.
}

/* Decodes a single instruction (passed in as a 32 bit integer) and stores its results in memory. */
func decode(instr uint32) {
	opcode := (instr >> 21) & 0x7FF
	rd := instr & 0x1F
	rn := (instr >> 5) & 0x1F
	rm := (instr >> 16) & 0x1F
	shamt := (instr >> 10) & 0x3F
	immediate := (instr >> 10) & 0xFFF
	bAddress := int16(instr & 0x3FFFFFF)
	cbAddress := int16((instr >> 5) & 0x7FFFF)
	dAddress := int16((instr >> 12) & 0x1FF)

	decodedInstr := DecodedInstruction{ opcode, rd, rn, rm, shamt, immediate, bAddress, cbAddress, dAddress }
	instructions[instructionIndex] = decodedInstr

	if printProgram {
		fmt.Printf("%b\t%d\n", instr, opcode)
		return
	}

	// Identify branches while decoding.
	if (opcode >= 160 && opcode <= 191) || (opcode >= 1184 && opcode <= 1215) { // B, BL
		branchIndex := instructionIndex + int(bAddress)
		labels[branchIndex] = true
	} else if (opcode >= 672 && opcode <= 679) || (opcode >= 1448 && opcode <= 1455) || (opcode >= 1440 && opcode <= 1447) {  // B.cond, CBNZ, CBZ
		branchIndex := instructionIndex + int(cbAddress)
		labels[branchIndex] = true
	}

	instructionIndex++
}

/* Prints the resulting program assembly, line-by-line, after all instructions have been loaded into memory. */
func printAssembly() {
	labelMap := map[int]string {}
	labelIndex := 0
	
	// Before printing instructions, we must identify our procedures and assign them labels.
	for i, _ := range instructions {
		if labels[i] {
			labelMap[i] = fmt.Sprintf("label%d", labelIndex)
			labelIndex++
		}
	}

	for i, instr := range instructions {
		if labels[i] { // If this instruction represents a procedure, we must first print its label.
			fmt.Println(labelMap[i] + ":")
		}

		// Decode LEGv8 instructions by their opcode range (in decimal).
		if instr.opcode == 1112 { // ADD
			fmt.Printf("ADD X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1160 || instr.opcode == 1161 { // ADDI
			fmt.Printf("ADDI X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode == 1104 { // AND
			fmt.Printf("AND X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1168 || instr.opcode == 1169 { // ANDI
			fmt.Printf("ANDI X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode >= 160 && instr.opcode <= 191 { // B
			fmt.Printf("B %s\n", labelMap[i + int(instr.bAddress)])
		} else if instr.opcode >= 672 && instr.opcode <= 679 { // B.cond
			cond, err := getCondForOpcode(instr.rd)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}

			fmt.Printf("B.%s %s\n", cond, labelMap[i + int(instr.cbAddress)])
		} else if instr.opcode >= 1184 && instr.opcode <= 1215 { // BL
			fmt.Printf("BL %s\n", labelMap[i + int(instr.bAddress)])
		} else if instr.opcode == 1712 { // BR
			fmt.Printf("BR X%d\n", instr.rn)
		} else if instr.opcode >= 1448 && instr.opcode <= 1455 { // CBNZ
			fmt.Printf("CBNZ X%d, %s\n", instr.rd, labelMap[i + int(instr.cbAddress)])
		} else if instr.opcode >= 1440 && instr.opcode <= 1447 { // CBZ
			fmt.Printf("CBZ X%d, %s\n", instr.rd, labelMap[i + int(instr.cbAddress)])
		} else if instr.opcode == 1616 { // EOR
			fmt.Printf("EOR X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1680 || instr.opcode == 1681 { // EORI
			fmt.Printf("EORI X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode == 1986 { // LDUR
			fmt.Printf("LDUR X%d, [X%d, #%d]\n", instr.rd, instr.rn, instr.dAddress)
		} else if instr.opcode == 1691 { // LSL
			fmt.Printf("LSL X%d, X%d, #%d\n", instr.rd, instr.rn, instr.shamt)
		} else if instr.opcode == 1690 { // LSR
			fmt.Printf("LSR X%d, X%d, #%d\n", instr.rd, instr.rn, instr.shamt)
		} else if instr.opcode == 1360 { // ORR
			fmt.Printf("ORR X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1424 || instr.opcode == 1425 { // ORRI
			fmt.Printf("ORRI X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode == 1984 { // STUR
			fmt.Printf("STUR X%d, [X%d, #%d]\n", instr.rd, instr.rn, instr.dAddress)
		} else if instr.opcode == 1624 { // SUB
			fmt.Printf("SUB X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1672 || instr.opcode == 1673 { // SUBI
			fmt.Printf("SUBI X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode == 1928 || instr.opcode == 1929 { // SUBIS
			fmt.Printf("SUBIS X%d, X%d, #%d\n", instr.rd, instr.rn, instr.immediate)
		} else if instr.opcode == 1880 { // SUBS
			fmt.Printf("SUBS X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)
		} else if instr.opcode == 1240 { // MUL (TODO verify)
			fmt.Printf("MUL X%d, X%d, X%d\n", instr.rd, instr.rn, instr.rm)

		// Psuedo-instructions
		} else if instr.opcode == 2045 { // PRNT
			fmt.Printf("PRNT X%d\n", instr.rd)
		} else if instr.opcode == 2044 { // PRNL
			fmt.Printf("PRNL\n")
		} else if instr.opcode == 2046 { // DUMP
			fmt.Printf("DUMP\n")
		} else if instr.opcode == 2047 { // HALT
			fmt.Printf("HALT\n")
		} else {
			fmt.Printf("Unhandled instruction; opcode in decimal: %d\n", instr.opcode)
		}
	}
}

/* Retrieves the string-representation of <cond> in a B.cond instruction. */
func getCondForOpcode(opcode uint32) (string, error) {
	switch opcode {
		case 0x0: return "EQ", nil
		case 0x1: return "NE", nil
		case 0x2: return "HS", nil
		case 0x3: return "LO", nil
		case 0x4: return "MI", nil
		case 0x5: return "PL", nil
		case 0x6: return "VS", nil
		case 0x7: return "VC", nil
		case 0x8: return "HI", nil
		case 0x9: return "LS", nil
		case 0xA: return "GE", nil
		case 0xB: return "LT", nil
		case 0xC: return "GT", nil
		case 0xD: return "LE", nil

		// Instruction field must be incorrect; print error.
		default: return "", errors.New(fmt.Sprintf("Unhandled <cond> opcode (%d) for instruction B.cond", opcode))
	}
}