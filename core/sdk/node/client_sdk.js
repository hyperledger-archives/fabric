var urlParser = require("url");

/**
 * Create a chain client mgr
 */
function NewChainClientMgr() {
   return new ChainClientMgr();
}

/**
 * Constructor for the chain client manager to manage one or more chain clients
 */
function ChainClientMgr() {
   this.chains = {};
   this.defaultTimeout = 60 * 1000;  // 1 minute
   // If running in bluemix, initialize from VCAP_SERVICES environment variable
   if (process.env.VCAP_SERVICES) {
      // TODO: From marbles app   
   }
}

/**
 * Get a chain.  If it doesn't yet exist and 'create' is true, create it.
 * @param {string} chainName The name of the chain to get or create.
 * @param {boolean} create If the chain doesn't already exist, specifies whether to create it.
 * @return {Chain} Returns the chain, or null if it doesn't exist and create is false.
 */
ChainClientMgr.prototype.getChain = function (chainName, create) {
   var chain = this.chains[chainName];
   if (!chain && create) {
      chain = new Chain(chainName,this);
      this.chains[chainName] = chain;
   }
   return chain;
};

/**
 * Get the default timeout in milliseconds.
 * @return {int} Returns the default timeout in milliseconds.
 */
ChainClientMgr.prototype.getDefaultTimeoutInMs = function () {
   return this.timeout;
};

/**
 * Set the default timeout in milliseconds.
 * @param {int} The default timeout milliseconds.
 */
ChainClientMgr.prototype.setDefaultTimeoutInMs = function (timeout) {
   this.timeout = timeout;
};

/**
 * Shutdown/cleanup everything related to the chain manager
 */
ChainClientMgr.prototype.shutdown = function () {
   var self = this;
   // Shutdown each chain
   for (chainName in self.chains) {
      self.chains[chainName].shutdown();
   }
};

/**
 * The chain constructor.
 * @param {string} name The name of the chain.  This can be any name that the client chooses.
 * @param {ChainClientMgr} mgr The manager used to create this chain.
 * @returns {ChainClient} A chain client
 */
function ChainClient(name,mgr) {
   this.name = name;
   this.mgr = mgr;
   this.peers = [];
}

/**
 * Get the chain name.
 * @returns {string} The name of the chain.
 */
ChainClient.prototype.getName = function() {
   return this.name;
};

/**
 * Get the chain manager
 * @returns {ChainClientMgr} The manager used to create this chain.
 */
ChainClient.prototype.getChainClientMgr = function() {
   return this.mgr;
};

/**
 * Add a peer given an endpoint specification.
 * @param {Object} endpoint The endpoint of the form: { url: "grpcs://host:port", tls: { .... } }
 * TBD: The format of 'endpoint.tls' depends upon the format expected by node's grpc module.  We may want to support Buffer and file path for convenience.
 * @returns {Peer} Returns a new peer.
 */
ChainClient.prototype.addPeer = function(endpoint) {
   var peer = new Peer(endpoint);
   this.peers.add(peer);
   return peer;
};

/**
 * Get the peers for this chain.
 */
ChainClient.prototype.getPeers  = function() {
   return this.peers;
};

/**
 * Set the member services config associated this chain.
 */
ChainClient.prototype.setMemberServices  = function(memberServices) {
   this.memberServices = memberServices;
};

/**
 * Constructor for a peer given the endpoint config for the peer.
 * @param {Object} config The endpoint config of the form: { url: "grpcs://host:port", tls: { .... } }
 * TBD: The format of 'config.tls' depends upon the format expected by node's grpc module.
 * @param {Chain} The chain of which this peer is a member.
 * @returns {Peer} The new peer.
 */
function Peer(endpoint,chain) {
   this.endpoint = endpoint;
   this.chain = chain;
}

/**
 * Get the chain of which this peer is a member.
 * @returns {Chain} The chain of which this peer is a member.
 */
Peer.prototype.getChain = function() {
   return this.chain;
};

/**
 * Get the URL of the peer.
 * @returns {string} Get the URL associated with the peer.
 */
Peer.prototype.getUrl = function() {

};

/**
 * Remove the peer from the chain.
 */
Peer.prototype.remove = function() {

};

function MemberServices(config) {

}

/**
 * Get a member by name.  The member
 */
MemberServices.prototype.getMember = function(name) {
   
};

/**
 * Registration request
 */
MemberServices.prototype.register = function(registrationRequest,cb) {
   
};

MemberServices.prototype.enroll = function(,cb) {
   
};

export.NewChainClientMgr = NewChainClientMgr;
