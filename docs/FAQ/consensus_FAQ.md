## Consensus Algorithm

&nbsp;
##### Which Consensus Algorithm is used in the fabric? 
The fabric is built on a pluggable architecture to enable developers to configure deployment with the consensus module that best suits their needs. The initial release package will offer three consensus implementations for users to select from: 1) no-op (consensus ignored); 2) classic PBFT, and 3) SIEVE (an enhanced version of classic PBFT). 

&nbsp;
##### What is the SIEVE Consensus Algorithm?
To manage the challenges presented by the security, performance, efficiency and scalability required of a for-business consensus algorithm, the team has been researching several approaches. One of them, called “SIEVE”, is a consensus algorithm inspired by classic PBFT [Castro and Liskov, OSDI’99] and the Eve consensus protocol [Kapritsos et al., OSDI’2012]. 

SIEVE augments the original PBFT algorithm by adding speculative execution and verification phases to detect and filter out possible non-deterministic requests and establish the determinism of transactions entering the PBFT 3-phase agreement protocol. SIEVE also allows consensus to be run on an output state of validators in addition to the consensus on their input state offered by classic PBFT. 

SIEVE is derived from PBFT in a modular way (inspired by ideas described in [Aublin et al., TOCS'15]) by reusing the PBFT view-change protocol. This lowers SIEVE's complexity and avoids having to develop a new consensus protocol from scratch. A paper will be published by IBM Research to describe this new algorithm in further detail. We're committed to making further research investments in the subject and sharing our results with the community.

