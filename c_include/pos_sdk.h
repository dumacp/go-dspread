#ifndef POS_SDK_H
#define POS_SDK_H

#include <stdio.h>
#define MAX_PACKAGE_LEN 1024

/**
//-------------greg.c
int setPosSleepTime(int time,char *out);
int setSystemDateTime(char* date,char*out);
int getMagneticTrackPlainText(char*out);
int isIdle();
int isQposPresent();
int cancelSetAmout();
int screenDisplay(char* display, int timeout,char*out);
int customInputDisplay(int operationType, int displayType, int maxLen, char* DisplayStr, int timeout,char* out);
int manualEnc(char* data, unsigned char len,int timeout,char* out);
int manualKsn(int ksnType,char* out);
int setEmvApp(int nIndex,int timeout,char* out);
int EmvApp(int nIndex,char* app);
int getAppCount();
int setMerchantId(char* MerchantId, char* out);
int setMerchantName(char* MerchantName, char* out);
int setIFDSerialNo(char* IFDSerial, char* out);

//------------- dspread.c
int readPublicRsa(char *out);
int packPosIdInfo(char *out);                                 // get pos id
int packPosDisplay(int operationtype,int displaytype,int maxlen,char*customdata,char *out);
int packPosInfo(char *out);                                   // get pos infomation
/** int packSwipeAndIC(int tradeMode,int tradeType,int MSRDebitCreditMode,char*TradeTime,int timeout, char *out); /**/
//int packSwipeAndIC(int tradeMode,int MSRDebitCreditMode,char*TradeTime,int timeout, char *out);
//int packSwipeAndIC(int tradeMode,int MSRDebitCreditMode,int timeout, char *out);                   // polling card  , if magnetic stripe card ,return transaction data. if IC card , call packSwipeIc() to get transaction result
/** int QF_packSwipeAndIC(int mode,int amount, char* random, char *extra, int timeout, char *out); /**/
/** int packSwipeIc(int tradeType,int amount, int cashback,char* tradeTime, char* tradeCurrencyCode,int timeout,char *out);   //if IC card,then return transaction result*/ /**/

/**/
int packQueryLatestCmdResult(char *out);                      // wait and query latest result
//int packWriteIc(char *script, char *outData);                 // if server validate transactin result and return . the write returned data to pos
/**/
int getIccTag(unsigned char encrptMode, unsigned char tagType,unsigned char tagCount, char *tagList,char* out);

int getCmdId(void);
//int get_dl_package(int cmd_id,int cmd_code, int cmd_sub_code, int delay,unsigned char* out);
int get(char* key,unsigned char *out);
//int resetposstatus(char *out);
/**
int getPin(char *trade_extra ,char *out );
int getPinBlock(int encryptType, int keyIndex, int maxLen, char * typeFace, char * cardNo, char * data, int waitPinTime,int timeout,char *out );
/**
void setAmount(char* Amount );
void setFormatID(char *format_id);
void setAmountIcon(char* AmountIcon );
void setTransMode(int mode);
int  sendpin(unsigned char* pin, unsigned char pinstatus, char* out);
int EMVSelectEMVApp(unsigned char index, char* out);
int is_package_receive_complete();
int packDigitalEnvelopeFragment(int offset,char* fragment,char* out);
int unpackDigitalEnvelope(char *out);
int update_work_key(char* pik, char* pikCheck, char* trk,char* trkCheck, char* mak, char* makCheck,int keyIndex,char* out);
int update_master_key(char* masterkey,char* masterkeyChekValue,int mkindex,char* out);
int doUpdateIPEKOperation(char* trackksn,char* trackipek,char* trackipekCheckvalue,char* emvksn,char* emvipek,char* emvipekCheckvalue,char* pinksn,char* pinipek,char* pinipekCheckvalue,int keyIndex,char* out);
/**/
int setBuzzerStatus(int isBuzzer,char *out);
void on_char(unsigned char c);
int on_package(unsigned char* p,int len);
int get_response_result();
void pack_string(char* key,unsigned char* value,int len);
void pack_u8(char* key,unsigned char value);
/**
void set_tck(unsigned char * new_key);
int testFunc();
/**/
int doMifare(int comCode,int timeout,char* out);
void setMifareKeyClass(int keyClass );
void setMifareBlockAddr(int addr );
void setMifareOperation(int cmd );
void setMifareKeyValue(char* keyValue );
void setMifareCardUid(char* cardUid );
void setMifareCardData(char* cardData );
void setMifareQuickAddr(int startAddr,int endAddr );
void mifareTranstransmission(char* data);

/**/

/***/
typedef enum _card_type_t
{
    CARD_NFC = 1,
    CARD_IC ,
    CARD_PSAM ,
}Card_Type_t;

int powerOnIcc(Card_Type_t cardtype,char encrptMode,unsigned int timeout ,char *out );

int sendApdu(char *cmd , unsigned int len ,unsigned int timeout ,char *out );

int powerOffIcc(unsigned int timeout ,char *out );

int powerOnNFC(char encrptMode,unsigned int timeout ,char *out );

int sendApduByNFC(char *cmd , unsigned int len ,unsigned int timeout ,char *out );

int powerOffNFC(unsigned int timeout ,char *out );

//int setMerchantId(char* MerchantId, char* out);

//int setMerchantName(char* MerchantName, char* out);
/* there are 2 methods to use algorithm.
 * 1. use the TMK TDK TPK which store in the PED.
 * 2.use the key in input params.  (if palgorithm_param_t->keyLen > 0)
*/
// int lib_Algorithm(palgorithm_param_t param, char* out);



void myitoa(int param_1,char *param_2,int base);


#endif
