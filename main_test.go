package main

import (
	"reflect"
	"testing"
)

func TestPdfStringToCsv(t *testing.T) {
	// Anonymized test data: replaced personal names, merchant names and IDs with generic placeholders while retaining date/amount patterns
	input := "ACCT TYPE X9999999999PERIOD TAG 2020HDRHDR / PURPOSEFIELDAMT (EUR)Valuta02.03.2020Dauerauftrag/Terminueberw.XUSR1-111,1102.03.2020Miete02.03.2020LastschriftVENDOR_A1-222,2202.03.2020DESCXPKG03/20VK:AAAAAAAAflowmodMandat:AAAAAAAA-02-1Referenz:AAAAAAAA02.03.2020LastschriftVENDOR_B2-333,33"
	expected := [][]string{
		{"02.03.2020", "Dauerauftrag/Terminueberw.XUSR1 - Miete", "-111,11"},
		{"02.03.2020", "LastschriftVENDOR_A1 - DESCXPKG03/20VK:AAAAAAAAflowmodMandat:AAAAAAAA-02-1Referenz:AAAAAAAA", "-222,22"},
		{"02.03.2020", "LastschriftVENDOR_B2", "-333,33"},
	}

	actual := pdfStringToCsv(input)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("pdfStringToCsv(%q) = %v; want %v", input, actual, expected)
	}

	input = "04.03.2020LastschriftMERCH_XY * TAG123-444,4404.03.2020RIDAAAAAAAAAAAAA-800-595SHOPFLAG02.0300000000TAILCODEóôÇÒÒÁõôòøñøöðõómãHerrnXUSR2Road 9 TownYProvider ZZ · 00000 TownDatum31.03.2020Auszugsnummer3Eingeräumte Kontoüberziehung0,00 EuroAlter Saldo11.222,33 EuroNeuer Saldo11.222,33 EuroIBANDE00 0000 0000 0000 0000 00BICABCDEFGXXXSeite1 von 5Provider ZZ · Demo-Str 5 · 00000 Town · Vorsitzender des Aufsichtsrates: Chair Person · Vorstand: M1 (Vorsitzender),M2 (stellv. Vorsitzender), M3, M4, M5, M6 · Sitz: Town · REG Town REG 0000,Steuernummer: 000 000 0000 0 · USt-IdNr.: DE 000 000 000 · Internet: www.example.test · E-Mail: info@example.test · BIC: ABCDEFGXXX · Mitglied im EinlagensicherungsfondsBuchungBuchung / VerwendungszweckBetrag (EUR)Valuta06.03.2020Dauerauftrag/Terminueberw.XUSR3-555,5506.03.2020SVCSUBSCR"

	expected = [][]string{
		{"04.03.2020", "LastschriftMERCH_XY * TAG123 - RIDAAAAAAAAAAAAA-800-595SHOPFLAG02.0300000000TAILCODE", "-444,44"},
		{"06.03.2020", "Dauerauftrag/Terminueberw.XUSR3 - SVCSUBSCR", "-555,55"},
	}

	actual = pdfStringToCsv(input)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("pdfStringToCsv(%q) = %v; want %v", input, actual, expected)
	}

	input = "31.03.2020SEQ.5555.SEQ.STORE_A,ItemBuySTORE_A,Art-100000000001Mandat:SEQMANDID1Referenz:SEQREF99999999SEQ.5555.SEQPAY31.03.2020LastschriftPROC_ENTITY LTD.-666,6631.03.2020SEQ.5555.SEQ.STORE_B,ItemBuySTORE_B,Art-200000000002Mandat:SEQMANDID1Referenz:SEQREF99999998SEQ.5555.SEQPAYNeuer Saldo35.267,63Kunden-InformationVorliegender Freistellungsauftrag0,00Bitte beachten Sie auch die Hinweise"

	expected = [][]string{
		{"31.03.2020", "SEQ.5555.SEQ.STORE_A,ItemBuySTORE_A,Art-100000000001Mandat:SEQMANDID1Referenz:SEQREF99999999SEQ.5555.SEQPAY - LastschriftPROC_ENTITY LTD. - SEQ.5555.SEQ.STORE_B,ItemBuySTORE_B,Art-200000000002Mandat:SEQMANDID1Referenz:SEQREF99999998SEQ.5555.SEQPAY", "-666,66"},
	}

	actual = pdfStringToCsv(input)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("pdfStringToCsv(%q) = %v; want %v", input, actual, expected)
	}

}
